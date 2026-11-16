package execution

import (
	"context"
	"fmt"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"forgeflow/internal/githubauth"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

var upstreamIssue = regexp.MustCompile(`^([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#([0-9]+)$`)

// TestRealUpstreamWorkflow runs the whole contribution pipeline on a real
// public repository and issue: a fresh clone, real Codex sessions, the real
// verification container, an isolated database. It proves what generated
// fixtures cannot, that ForgeFlow finishes work it would actually be offered.
// Nothing is pushed and no pull request is opened; the clone is disposable.
//
//	FORGEFLOW_REAL_UPSTREAM=owner/repo#123            the issue to work on
//	FORGEFLOW_TEST_CONTAINER_IMAGE=...                the verification image
//	FORGEFLOW_TEST_CONTRIBUTIONS_ROOT=S:\...          parent for the clone
//	FORGEFLOW_TEST_CI_SECONDS=240                     the repository's CI median
//	FORGEFLOW_TEST_EVIDENCE=dir                       where to export evidence
//	FORGEFLOW_ROOT=...                                project root, to reuse the saved GitHub sign-in
func TestRealUpstreamWorkflow(t *testing.T) {
	target := upstreamIssue.FindStringSubmatch(os.Getenv("FORGEFLOW_REAL_UPSTREAM"))
	if target == nil {
		t.Skip("set FORGEFLOW_REAL_UPSTREAM=owner/repo#number to run a real upstream contribution")
	}
	repository := target[1]
	number, _ := strconv.Atoi(target[2])
	ctx := context.Background()
	client := github.New("")
	if root := os.Getenv("FORGEFLOW_ROOT"); root != "" {
		account, err := githubauth.New(ctx, root, client, "")
		if err != nil {
			t.Fatal(err)
		}
		defer account.Close()
		for i := 0; i < 15 && !account.Status().Connected; i++ {
			time.Sleep(time.Second)
		}
	}
	session := &github.Session{Client: client, MaxRequests: 4}
	var issue github.Issue
	if err := session.Get(ctx, fmt.Sprintf("/repos/%s/issues/%d", repository, number), &issue); err != nil {
		t.Fatal(err)
	}
	if issue.State != "open" {
		t.Fatalf("%s#%d is %s", repository, number, issue.State)
	}
	cli := codex.Resolve("", "")
	cli.VerificationImage = os.Getenv("FORGEFLOW_TEST_CONTAINER_IMAGE")
	if mode := os.Getenv("FORGEFLOW_TEST_SANDBOX"); mode != "" {
		if err := cli.SetSandboxMode(mode); err != nil {
			t.Fatal(err)
		}
	}
	if base := os.Getenv("FORGEFLOW_TEST_CONTRIBUTIONS_ROOT"); base != "" {
		if !filepath.IsAbs(base) {
			t.Fatal("absolute disposable fixture parent required")
		}
		t.Setenv("TEMP", base)
		t.Setenv("TMP", base)
	}
	spec := fixtureSpec{Replace: true, Title: issue.Title, Body: issue.Body, Clone: "https://github.com/" + repository + ".git", Repository: repository, Number: number}
	if seconds, _ := strconv.Atoi(os.Getenv("FORGEFLOW_TEST_CI_SECONDS")); seconds > 0 {
		spec.CI = &domain.CIEvidence{SuccessfulRuns: 1, MedianSeconds: seconds, ObservedAt: time.Now().UTC()}
	}
	s, c := buildFixtureSpec(t, cli, os.Getenv("FORGEFLOW_TEST_MODEL"), false, spec)
	repo, _, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	if cli.VerificationImage != "" {
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := cli.RemoveRuntimeVolume(cleanup, repo); err != nil {
				t.Error(err)
			}
		})
	}
	started := time.Now()
	report := func() {
		record, _ := s.Store.Execution(ctx, c.ID)
		tests, _ := s.Store.Tests(ctx, c.ID)
		agents, _ := s.Store.Agents(ctx)
		t.Logf("%s#%d after %s: status %s, phase %s, %d fixes, %d reviews", repository, number, time.Since(started).Round(time.Second), record.Status, record.Phase, record.FixIterations, record.ReviewCycles)
		t.Logf("summary: %s", record.Summary)
		if record.Message != "" {
			t.Logf("message: %s", record.Message)
		}
		if i := record.Incident; i != nil {
			t.Logf("incident %s: %s\nevidence: %s\nnext: %s", i.Kind, i.Summary, i.Evidence, i.NextAction)
		}
		if b := record.Baseline; b != nil {
			t.Logf("baseline %s %s", b.Status, b.Reason)
		}
		for _, run := range tests {
			t.Logf("command %s %v -> exit %d in %s %s %s", run.Command.Program, run.Command.Arguments, run.ExitCode, run.FinishedAt.Sub(run.StartedAt).Round(time.Second), run.FailureKind, run.Error)
		}
		var input, output int64
		roles := map[string]int{}
		for _, a := range agents {
			if a.ContributionID == c.ID {
				roles[a.Role]++
				input += a.Usage["input_tokens"]
				output += a.Usage["output_tokens"]
			}
		}
		t.Logf("sessions %v; observed tokens in/out %d/%d; changed files %v", roles, input, output, record.ChangedFiles)
		if evidence := os.Getenv("FORGEFLOW_TEST_EVIDENCE"); evidence != "" {
			if err := os.MkdirAll(evidence, 0700); err != nil {
				t.Error(err)
				return
			}
			events, _ := s.Store.Events(ctx, 0, c.ID, 1000)
			if err := writeJSON(filepath.Join(evidence, "workflow.json"), map[string]any{"repository": repository, "issue": number, "workspace": repo, "execution": record, "tests": tests, "agents": agents, "events": events}); err != nil {
				t.Error(err)
			}
		}
	}
	defer report()
	// A real repository needs its public toolchain and dependencies, so the run
	// has the download approval a human would grant.
	if _, err := s.Start(ctx, c.ID, true, true); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	record, err := s.Store.Execution(ctx, c.ID)
	if err != nil || record.Status != "READY" {
		t.Fatalf("upstream contribution did not reach READY: status %s, %v", record.Status, err)
	}
	if record.Baseline == nil || record.Baseline.Status == "" || record.Baseline.Status == "unavailable" {
		t.Fatalf("baseline was not measured: %+v", record.Baseline)
	}
	if len(record.ChangedFiles) == 0 || record.Diff == "" {
		t.Fatal("READY without a change")
	}
}
