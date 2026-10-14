package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMixedAssertionsKeepIndependentSetupEvidence(t *testing.T) {
	for _, tc := range []struct{ evidence, kind string }{
		{"ModuleNotFoundError: No module named 'pytest'", "dependencies_missing"},
		{"exec: semgrep: executable file not found in $PATH", "tool_unavailable"},
		{"write /tmp/result: no space left on device", "disk_capacity"},
		{"fatal: detected dubious ownership in repository at '/workspace'", "environment_blocked"},
		{"open cache: permission denied", "workspace_permissions"},
		{"proxyconnect tcp: dial tcp 127.0.0.1:9: connection refused", "network_transport"},
		{"go: go.mod requires go >= 1.27.0", "toolchain_mismatch"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			output := "--- FAIL: TestDiagnosticText (0.01s)\n expected fatal: detected dubious ownership in repository, got nil\n--- FAIL: TestHelper (0.02s)\n " + tc.evidence + "\nFAIL\n"
			d := diagnoseCommand(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, output, errors.New("exit status 1"), true)
			if d.Kind != tc.kind {
				t.Fatalf("independent setup failure: got %s, want %s", d.Kind, tc.kind)
			}
			if !strings.Contains(d.Evidence, "expected fatal:") || !strings.Contains(d.Evidence, tc.evidence) {
				t.Fatal("original diagnostic evidence was lost", d.Evidence)
			}
		})
	}
}

func TestVersionFiveDiagnosisRefreshesWithoutChangingHistory(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	ctx := context.Background()
	command := domain.VerificationCommand{Program: "python3", Arguments: []string{"-m", "unittest"}}
	output := "--- FAIL: TestImport (0.01s)\nModuleNotFoundError: No module named 'pytest'"
	if err := s.Store.SaveTest(ctx, domain.TestRun{ID: "old-failure", ContributionID: c.ID, Command: command, ExitCode: 1, Output: output}); err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "NEEDS_INPUT", Network: true, Incident: &domain.ExecutionIncident{ID: "old-diagnosis", Kind: "code_failure", TestRunID: "old-failure", DiagnosisVersion: 5}}
	if err := s.Store.SaveExecution(ctx, r, "fixture", "historical diagnosis"); err != nil {
		t.Fatal(err)
	}
	before, err := s.Store.Events(ctx, 0, c.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.Diagnose(ctx, c.ID)
	if err != nil || d == nil || d.Kind != "dependencies_missing" || d.DiagnosisVersion != diagnosisVersion || d.TestRunID != "old-failure" {
		t.Fatalf("old diagnosis was not refreshed: %+v %v", d, err)
	}
	saved, err := s.Store.Execution(ctx, c.ID)
	if err != nil || saved.Incident.Kind != "code_failure" || saved.Incident.DiagnosisVersion != 5 || saved.Status != "NEEDS_INPUT" {
		t.Fatal("read-only diagnosis changed saved history", saved, err)
	}
	after, err := s.Store.Events(ctx, 0, c.ID, 100)
	if err != nil || len(after) != len(before) {
		t.Fatal("diagnosis rewrote audit events", err)
	}
}

func TestPlannerSubsetKeepsRequiredChecks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"node tests.js","lint":"eslint .","build":"vite build","typecheck":"tsc --noEmit"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	commands, err := verificationCommands(dir, &domain.Plan{Tests: []domain.VerificationCommand{{Program: "npm", Arguments: []string{"run", "typecheck"}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"test", "lint", "build", "typecheck"} {
		found := false
		for _, command := range commands {
			if command.Program == "npm" && strings.Join(command.Arguments, " ") == "run "+name {
				found = true
			}
		}
		if !found {
			t.Errorf("required npm run %s disappeared: %+v", name, commands)
		}
	}
}

type resumedReviewRunner struct {
	fixtureRunner
	commands int
}

func (f *resumedReviewRunner) Command(ctx context.Context, dir string, command domain.VerificationCommand, network bool) (domain.TestRun, error) {
	f.commands++
	return f.fixtureRunner.Command(ctx, dir, command, network)
}

func TestResumedReviewTestsChangedPatch(t *testing.T) {
	f := &resumedReviewRunner{fixtureRunner: fixtureRunner{reviews: 1}}
	s, c := buildFixture(t, f)
	repo, meta, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = s.prepareRuntime(repo); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(repo, "sum.go")
	if err = os.WriteFile(source, []byte("package fixture\nfunc Add(a,b int) int{return a+b}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command := domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}
	passed, err := f.Command(ctx, repo, command, false)
	if err != nil || passed.ExitCode != 0 {
		t.Fatalf("real good-patch baseline: %v %s", err, passed.Output)
	}
	passed.ID, passed.ContributionID = "verified-good-patch", c.ID
	if err = s.Store.SaveTest(ctx, passed); err != nil {
		t.Fatal(err)
	}
	goodDiff, _, err := s.diff(ctx, repo, c.BaseCommit)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, []byte("package fixture\n// Changed after verification.\nfunc Add(a,b int) int{return a-b}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := s.Store.Approval(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID, Phase: "REVIEWING", Plan: &domain.Plan{Summary: "Fix addition", Tests: []domain.VerificationCommand{command}}, Diff: goodDiff}
	if err = s.Store.SaveExecutionCheckpoints(ctx, r, []string{"PLANNING", "CODING", "TESTING", "REVIEWING"}, "fixture", "interrupted review"); err != nil {
		t.Fatal(err)
	}
	f.commands = 0
	if err = s.workflow(ctx, c, a, &r, repo, meta); err != nil {
		t.Fatal(err)
	}
	if f.commands == 0 || r.Status != "READY" {
		t.Fatalf("changed patch was not verified: commands=%d status=%s", f.commands, r.Status)
	}
	runs, err := s.Store.Tests(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundFailure := false
	for _, run := range runs {
		if run.ExitCode != 0 {
			foundFailure = true
		}
	}
	if !foundFailure || runs[len(runs)-1].ExitCode != 0 {
		t.Fatal("real failed and passing retests were not retained", runs)
	}
}
