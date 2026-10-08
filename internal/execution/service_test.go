package execution

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/config"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixtureRunner struct {
	contributors, reviews int
	roles                 []string
	models                []string
	alwaysFail            bool
	blockedVerification   bool
}

func (f *fixtureRunner) Check(context.Context, string, string) error { return nil }
func (f *fixtureRunner) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	f.roles = append(f.roles, r.Role)
	f.models = append(f.models, r.Model)
	if event != nil {
		if e := event(json.RawMessage(`{"type":"turn.started"}`)); e != nil {
			return codex.Result{}, e
		}
	}
	var value any
	switch r.Role {
	case "planner":
		value = domain.Plan{Summary: "Correct addition and add regression tests", RootCause: "Subtracting the second operand", Files: []string{"sum.go", "sum_test.go"}, Strategy: "Use addition and test positive and negative operands", Tests: []domain.VerificationCommand{{Program: "go", Arguments: []string{"test", "./..."}}}, Risks: []string{}, Unknowns: []string{}}
	case "contributor":
		f.contributors++
		if f.contributors == 1 {
			os.WriteFile(filepath.Join(r.Directory, "sum.go"), []byte("package fixture\n// Addition fix in progress.\nfunc Add(a,b int) int {return a-b}\n"), 0600)
		} else {
			os.WriteFile(filepath.Join(r.Directory, "sum.go"), []byte("package fixture\nfunc Add(a,b int) int {return a+b}\n"), 0600)
		}
		if f.contributors >= 3 {
			os.WriteFile(filepath.Join(r.Directory, "negative_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestNegative(t *testing.T){if Add(-2,-3)!=-5 {t.Fatal(\"negative sum\")}}\n"), 0600)
		}
		value = map[string]string{"status": "IMPLEMENTED", "summary": "Fixed addition with regression tests"}
		if f.blockedVerification && f.contributors >= 2 {
			value = map[string]string{"status": "BLOCKED", "summary": "Fixed the patch. Agent testing stalled on downloads and Docker is unavailable; required verification remains blocked."}
		}
	case "reviewer":
		f.reviews++
		review := domain.Review{Verdict: "APPROVE", Summary: "Correct minimal fix with positive and negative regression coverage", Findings: []domain.Finding{}}
		if f.reviews == 1 {
			review.Verdict = "REQUEST_CHANGES"
			review.Findings = []domain.Finding{{Severity: "medium", File: "sum_test.go", Line: 3, Explanation: "Missing negative operand coverage", Fix: "Add a negative operand regression test"}}
		}
		value = review
	}
	b, _ := json.Marshal(value)
	return codex.Result{SessionID: storage.ID(), Output: string(b), Usage: map[string]int64{"input_tokens": 10, "output_tokens": 5}}, nil
}
func (f *fixtureRunner) Command(ctx context.Context, dir string, c domain.VerificationCommand, network bool) (domain.TestRun, error) {
	rec := domain.TestRun{Command: c, Directory: dir, StartedAt: time.Now().UTC(), ExitCode: -1}
	cmd := exec.CommandContext(ctx, c.Program, c.Arguments...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	rec.FinishedAt = time.Now().UTC()
	rec.Output = string(out)
	if cmd.ProcessState != nil {
		rec.ExitCode = cmd.ProcessState.ExitCode()
	}
	if f.alwaysFail {
		rec.ExitCode = 1
		err = errors.New("synthetic repeated failure")
	}
	return rec, err
}
func buildFixture(t *testing.T, runner codex.Runner) (*Service, domain.Contribution) {
	return buildFixtureWithModel(t, runner, "")
}
func buildFixtureWithModel(t *testing.T, runner codex.Runner, model string) (*Service, domain.Contribution) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, ".cache"), 0700)
	base := filepath.Join(root, "external")
	repo := filepath.Join(base, "fixture", "repo")
	meta := filepath.Join(base, "fixture", ".autopilot")
	for _, path := range []string{repo, meta, filepath.Join(meta, "empty-hooks"), filepath.Join(base, "fixture", "artifacts")} {
		if e := os.MkdirAll(path, 0700); e != nil {
			t.Fatal(e)
		}
	}
	for name, body := range map[string]string{"go.mod": "module example.com/fixture\n\ngo 1.26\n", "sum.go": "package fixture\nfunc Add(a,b int) int {return a-b}\n", "sum_test.go": "package fixture\nimport \"testing\"\nfunc TestAdd(t *testing.T){if Add(2,3)!=5 {t.Fatal(\"addition incorrect\")}}\n"} {
		if e := os.WriteFile(filepath.Join(repo, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = gitEnv()
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("fixture Git: %v %s", e, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "initial")
	sha := git("rev-parse", "HEAD")
	git("checkout", "-b", "autopilot/issue-1")
	store, e := storage.Open(ctx, filepath.Join(root, "test.db"), false)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { store.Close() })
	start := time.Now().UTC()
	run := domain.DiscoveryRun{ID: "fixture-scan", Status: "RUNNING", ConfigVersion: 1, StartedAt: start}
	if e = store.StartDiscovery(ctx, run); e != nil {
		t.Fatal(e)
	}
	end := time.Now().UTC()
	run.Status = "SUCCEEDED"
	run.FinishedAt = &end
	o := domain.Opportunity{ID: "fixture-issue", Repository: "example/fixture", Number: 1, Title: "Correct Add", Evidence: &domain.Evidence{RunID: run.ID, ConfigVersion: 1}}
	if e = store.FinishDiscovery(ctx, run, []domain.Opportunity{o}); e != nil {
		t.Fatal(e)
	}
	c := domain.Contribution{ID: "fixture", OpportunityID: o.ID, Repository: o.Repository, Title: o.Title, State: "SELECTED", Branch: "autopilot/issue-1", BaseCommit: sha, Workspace: filepath.ToSlash(repo), ConfigVersion: 1, CodexModel: model}
	cfg := domain.ConfigVersion{Version: 1, Config: config.Default()}
	cfg.Config.Codex.MaxFixIterations = 3
	cfg.Config.Codex.MaxReviewCycles = 2
	a := domain.WorkspaceApproval{Token: "fixture-approval", Config: cfg, Opportunity: o, Issue: json.RawMessage(`{"number":1,"state":"open","title":"Correct Add","body":"The Add function subtracts instead of adding. Fix addition and add positive and negative regression coverage."}`), Repository: json.RawMessage(`{"full_name":"example/fixture","default_branch":"main"}`)}
	if e = store.ApproveWorkspace(ctx, c, a); e != nil {
		t.Fatal(e)
	}
	for _, state := range []string{"PREPARING", "ANALYZING_REPOSITORY", "PLANNING", "BLOCKED"} {
		if e = store.Transition(ctx, c.ID, state); e != nil {
			t.Fatal(e)
		}
	}
	return New(ctx, store, runner, nil, root, base), c
}
func TestWorkflowRealCommandsFixIndependentReviewAndPRGate(t *testing.T) {
	runner := &fixtureRunner{}
	s, c := buildFixtureWithModel(t, runner, "model-a")
	ctx := context.Background()
	if _, e := s.Start(ctx, c.ID, false, false); e == nil {
		t.Fatal("execution started without approval")
	}
	if _, e := s.Start(ctx, c.ID, true, false); e != nil {
		t.Fatal(e)
	}
	s.Wait()
	current, _ := s.Store.Contribution(ctx, c.ID)
	r, e := s.Store.Execution(ctx, c.ID)
	if e != nil || current.State != "READY" {
		t.Fatalf("workflow did not finish: %+v %+v %v", current, r, e)
	}
	if r.FixIterations != 2 || r.ReviewCycles != 2 || r.Review.Verdict != "APPROVE" {
		t.Fatalf("missing bounded loops: %+v", r)
	}
	tests, _ := s.Store.Tests(ctx, c.ID)
	failed := false
	for _, run := range tests {
		if run.ExitCode != 0 {
			failed = true
		}
	}
	if !failed || tests[len(tests)-1].ExitCode != 0 {
		t.Fatal("real failure and recovery evidence missing")
	}
	agents, _ := s.Store.Agents(ctx)
	sessions := map[string]bool{}
	for _, a := range agents {
		if a.Status != "SUCCEEDED" || sessions[a.SessionID] || a.Model != "model-a" {
			t.Fatal("contexts not independent")
		}
		sessions[a.SessionID] = true
	}
	for _, model := range runner.models {
		if model != "model-a" {
			t.Fatalf("agent received wrong model: %q", model)
		}
	}
	if !strings.Contains(r.Diff, "negative_test.go") || r.Report == "" {
		t.Fatal("new file or report missing")
	}
	if _, e = s.SubmitPR(ctx, c.ID, "", false); e == nil {
		t.Fatal("PR submitted without approval")
	}
	prepared, e := s.PreparePR(ctx, c.ID)
	if e != nil || prepared.HeadCommit == c.BaseCommit || prepared.SubmissionToken == "" {
		t.Fatalf("local preparation failed: %+v %v", prepared, e)
	}
	repo, _, _ := s.Paths(c)
	committedDiff, _, e := s.diff(ctx, repo, c.BaseCommit)
	if e != nil || committedDiff != r.Diff {
		t.Fatal("reviewed diff changed when new files were committed", e)
	}
	if _, e = s.SubmitPR(ctx, c.ID, prepared.SubmissionToken, false); e == nil {
		t.Fatal("submission gate bypassed")
	}
	current, _ = s.Store.Contribution(ctx, c.ID)
	if current.State != "PR_PREPARED" {
		t.Fatal("PR opened unexpectedly")
	}
}
func TestRepeatedFailuresStopAtConfiguredBound(t *testing.T) {
	f := &fixtureRunner{alwaysFail: true}
	s, c := buildFixture(t, f)
	ctx := context.Background()
	if _, e := s.Start(ctx, c.ID, true, false); e != nil {
		t.Fatal(e)
	}
	s.Wait()
	r, _ := s.Store.Execution(ctx, c.ID)
	if r.Status != "BLOCKED" || r.FixIterations != 3 || !strings.Contains(r.Summary, "maximum fix") {
		t.Fatalf("retry bound not enforced: %+v", r)
	}
}
func TestRealCodexWorkflow(t *testing.T) {
	if os.Getenv("FORGEFLOW_REAL_CODEX_TEST") != "1" {
		t.Skip("opt in to actual authenticated Codex sessions")
	}
	cli := codex.Resolve("", "")
	s, c := buildFixture(t, cli)
	ctx := context.Background()
	if _, e := s.Start(ctx, c.ID, true, false); e != nil {
		t.Fatal(e)
	}
	s.Wait()
	r, e := s.Store.Execution(ctx, c.ID)
	if e != nil || r.Status != "READY" {
		t.Fatalf("actual Codex workflow: %+v %v", r, e)
	}
	tests, _ := s.Store.Tests(ctx, c.ID)
	if len(tests) == 0 {
		t.Fatal("no real verification")
	}
	t.Logf("actual Codex READY: %d commands, %d fixes, %d reviews", len(tests), r.FixIterations, r.ReviewCycles)
}

type waitingRunner struct {
	fixtureRunner
	started chan struct{}
}

func TestAgentCannotChangeGitConfigForUnsandboxedHostOperations(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	repo, _, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.prepareRuntime(repo); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(repo, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("\n[filter \"unsafe\"]\n clean = arbitrary-command\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.git(context.Background(), repo, "status", "--porcelain"); err == nil {
		t.Fatal("modified Git configuration was accepted")
	}
	if err = s.prepareRuntime(repo); err == nil {
		t.Fatal("resume replaced the pinned configuration snapshot")
	}
}

func (f *waitingRunner) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	close(f.started)
	<-ctx.Done()
	return codex.Result{}, ctx.Err()
}

func TestPausePreservesWorkspaceAndRequiresConfirmedResume(t *testing.T) {
	runner := &waitingRunner{started: make(chan struct{})}
	s, c := buildFixture(t, runner)
	ctx := context.Background()
	if err := s.SetConstraints(ctx, c.ID, "Preserve the public API", false); err == nil {
		t.Fatal("unconfirmed constraints accepted")
	}
	if err := s.SetConstraints(ctx, c.ID, "Preserve the public API", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-runner.started:
	case <-time.After(10 * time.Second):
		t.Fatal("agent did not start")
	}
	if err := s.SetConstraints(ctx, c.ID, "Change API", true); err == nil {
		t.Fatal("constraints changed during execution")
	}
	if err := s.Control(ctx, c.ID, "pause", false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	current, err := s.Store.Contribution(ctx, c.ID)
	if err != nil || current.State != "PAUSED" {
		t.Fatal("pause did not persist", current, err)
	}
	record, err := s.Store.Execution(ctx, c.ID)
	if err != nil || record.Constraints != "Preserve the public API" || record.Status != "PAUSED" {
		t.Fatal("constraints/evidence lost", record, err)
	}
	if _, err := s.Start(ctx, c.ID, false, false); err == nil {
		t.Fatal("unconfirmed resume allowed")
	}
	repo, _, err := s.Paths(current)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(repo, "sum.go")); err != nil {
		t.Fatal("workspace lost on pause", err)
	}
	if err = s.Store.SaveExecution(ctx, domain.ExecutionRecord{ContributionID: c.ID, Status: "AWAITING_PLAN_APPROVAL", Phase: "PLANNING", Plan: &domain.Plan{Summary: "Review first"}}, "test", "fixture plan"); err != nil {
		t.Fatal(err)
	}
	if err = s.Control(ctx, c.ID, "approve-plan", false); err == nil {
		t.Fatal("plan approval bypassed")
	}
	if err = s.Control(ctx, c.ID, "approve-plan", true); err != nil {
		t.Fatal(err)
	}
	record, err = s.Store.Execution(ctx, c.ID)
	if err != nil || !record.PlanApproved || record.Status != "PAUSED" {
		t.Fatal("approved plan cannot resume", record, err)
	}
}
