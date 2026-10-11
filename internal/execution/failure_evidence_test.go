package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const signalPipeFailure = "--- FAIL: TestSetupGitPinsTheIdentityGitAuthenticatesAs (4.18s)\n    git_credential_identity_test.go:112: git credential fill: exit status 128\n        sh.exe: *** fatal error - couldn't create signal pipe, Win32 error 5\n        fatal: could not read Username for 'https://bitbucket.example.com': terminal prompts disabled\nFAIL"
const generatedPathFailure = "FAIL\t./... [setup failed]\n# ./...\npattern ./...: open bin\\praetor-dev-mcp-2_dbv0av: Access is denied.\nFAIL"

func TestObservedWindowsFailuresHaveCausalDiagnoses(t *testing.T) {
	cmd := domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}
	for _, tc := range []struct {
		output, kind string
		err          error
	}{
		{signalPipeFailure, "sandbox_process", errors.New("exit status 1")},
		{generatedPathFailure, "generated_path_access", nil},
		{"--- FAIL: TestAuth\nexpected 401 Unauthorized, got 200", "code_failure", nil},
		{"--- FAIL: TestFile\nexpected permission denied, got nil", "code_failure", nil},
		{"--- FAIL: TestLong\nwaiting for service", "test_failure_timeout", context.DeadlineExceeded},
	} {
		d := diagnoseCommand(cmd, tc.output, tc.err, true)
		if d.Kind != tc.kind || !strings.Contains(d.Evidence, tc.output) || d.DiagnosisVersion != diagnosisVersion {
			t.Fatalf("%s: %+v", tc.kind, d)
		}
	}
}

func TestPackageDiscoveryRecoversEmptyGeneratedDirectoryWithoutChangingTests(t *testing.T) {
	f := &reactorRunner{outputs: []string{generatedPathFailure, ""}}
	s, c := buildFixture(t, f)
	repo, meta, _ := s.Paths(c)
	if err := s.prepareRuntime(repo); err != nil {
		t.Fatal(err)
	}
	denied := filepath.Join(repo, "bin", "praetor-dev-mcp-2_dbv0av")
	if err := os.MkdirAll(denied, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("/bin/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING", Phase: "TESTING", Plan: &domain.Plan{}}
	passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
	if err != nil || !passed || r.FixIterations != 0 || r.RecoveryAttempts != 1 || f.contributors != 0 {
		t.Fatalf("recovery failed: %+v %v", r, err)
	}
	if _, err := os.Stat(denied); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("directory still blocks package scan", err)
	}
	paths, err := filepath.Glob(filepath.Join(repo, ".forgeflow-runtime", "quarantine-*"))
	if err != nil || len(paths) != 1 {
		t.Fatal("original directory not preserved", paths, err)
	}
	runs, _ := s.Store.Tests(context.Background(), c.ID)
	if len(runs) != 3 || runs[0].ExitCode == 0 || runs[1].RecoveryOf != runs[0].ID || !reflect.DeepEqual(runs[0].Command, runs[1].Command) {
		t.Fatalf("lost original verification: %+v", runs)
	}
	if err := s.verifyApprovedGit(context.Background(), repo, c.BaseCommit, c.Branch); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedPathRepairRefusesSourceNonemptyUnignoredAndEscapingPaths(t *testing.T) {
	s, c := buildFixture(t, &reactorRunner{})
	repo, _, _ := s.Paths(c)
	if err := s.prepareRuntime(repo); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bin/occupied", "bin/unignored", "src/empty"} {
		if err := os.MkdirAll(filepath.Join(repo, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	content := filepath.Join(repo, "bin", "occupied", "keep.txt")
	if err := os.WriteFile(content, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"bin/occupied", "bin/unignored", "src/empty", "bin/../../outside", "bin/.git", "S:/outside", "//server/share", "bin"} {
		r := domain.ExecutionRecord{ContributionID: c.ID}
		err := s.quarantineGeneratedDirectory(context.Background(), c.ID, repo, &r, &domain.ExecutionIncident{Path: path})
		if err == nil || r.RecoveryAttempts != 0 {
			t.Fatalf("unsafe repair %s: %v", path, err)
		}
	}
	b, err := os.ReadFile(content)
	if err != nil || string(b) != "preserve" {
		t.Fatal("file changed", err)
	}
}

func TestLegacyDiagnosisUsesSavedCommandWithoutRewritingHistory(t *testing.T) {
	for _, tc := range []struct{ old, output, want string }{{"authentication", signalPipeFailure, "sandbox_process"}, {"workspace_permissions", generatedPathFailure, "generated_path_access"}} {
		t.Run(tc.want, func(t *testing.T) {
			s, c := buildFixture(t, &reactorRunner{})
			ctx := context.Background()
			run := domain.TestRun{ID: "legacy-test", ContributionID: c.ID, Command: domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, ExitCode: 1, Output: tc.output}
			if err := s.Store.SaveTest(ctx, run); err != nil {
				t.Fatal(err)
			}
			r := domain.ExecutionRecord{ContributionID: c.ID, Status: "BLOCKED", Phase: "TESTING", Incident: &domain.ExecutionIncident{ID: "old", Kind: tc.old, TestRunID: run.ID, Evidence: "old evidence"}}
			if err := s.Store.SaveExecution(ctx, r, "fixture", "old failure"); err != nil {
				t.Fatal(err)
			}
			d, err := s.Diagnose(ctx, c.ID)
			if err != nil || d.Kind != tc.want || d.TestRunID != run.ID || !strings.Contains(d.Evidence, tc.output) {
				t.Fatalf("wrong diagnosis: %+v %v", d, err)
			}
			saved, _ := s.Store.Execution(ctx, c.ID)
			if saved.Incident.Kind != tc.old || saved.Status != "BLOCKED" {
				t.Fatal("diagnosis mutated history", saved)
			}
		})
	}
}

func TestPersistentShellFailureNeverEntersCodeFixLoop(t *testing.T) {
	f := &reactorRunner{outputs: []string{signalPipeFailure, signalPipeFailure, signalPipeFailure}}
	s, c := buildFixture(t, f)
	ctx := context.Background()
	r := domain.ExecutionRecord{ContributionID: c.ID, Phase: "TESTING", Plan: &domain.Plan{Summary: "Saved plan"}, Network: true}
	if err := s.Store.SaveExecutionCheckpoints(ctx, r, []string{"PLANNING", "CODING", "TESTING", "BLOCKED"}, "fixture", "saved patch"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(ctx, c.ID, true, true); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	saved, _ := s.Store.Execution(ctx, c.ID)
	if saved.Status != "NEEDS_INPUT" || saved.Incident.Kind != "sandbox_process" || saved.FixIterations != 0 || f.contributors != 0 || f.reviews != 0 || len(f.commands) != 3 {
		t.Fatalf("wrong recovery: %+v, commands %d", saved, len(f.commands))
	}
	if !strings.Contains(saved.Incident.NextAction, "compatible Codex Windows sandbox") {
		t.Fatal("lost actionable cause", saved.Incident)
	}
}

type timeoutEvidenceRunner struct{ reactorRunner }

func (f *timeoutEvidenceRunner) Command(context.Context, string, domain.VerificationCommand, bool) (domain.TestRun, error) {
	return domain.TestRun{ExitCode: 1, Output: "--- FAIL: TestSlow\npartial test output"}, context.DeadlineExceeded
}
func TestRunnerTimeoutSurvivesPersistenceAlongsidePartialOutput(t *testing.T) {
	s, c := buildFixture(t, &timeoutEvidenceRunner{})
	repo, _, _ := s.Paths(c)
	run, err := s.recordedCommand(context.Background(), c.ID, repo, domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, false, "")
	if !errors.Is(err, context.DeadlineExceeded) || run.FailureKind != "test_failure_timeout" {
		t.Fatal(run, err)
	}
	runs, e := s.Store.Tests(context.Background(), c.ID)
	if e != nil || len(runs) != 1 || !errors.Is(savedCommandError(runs[0]), context.DeadlineExceeded) || !strings.Contains(runs[0].Output, "partial test output") {
		t.Fatal("lost timeout evidence", runs, e)
	}
}

type brokenShellRunner struct {
	reactorRunner
	probes int
}

func (f *brokenShellRunner) ProbeGitShell(context.Context, string) (domain.TestRun, error) {
	f.probes++
	return domain.TestRun{ExitCode: 66, Output: "sh.exe: couldn't create signal pipe, Win32 error 5"}, errors.New("exit status 66")
}
func TestKnownBrokenShellUsesShortProbesInsteadOfRepeatingBuild(t *testing.T) {
	f := &brokenShellRunner{}
	s, c := buildFixture(t, f)
	ctx := context.Background()
	run := domain.TestRun{ID: "previous-shell-failure", ContributionID: c.ID, Command: domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, ExitCode: 1, Output: signalPipeFailure}
	if err := s.Store.SaveTest(ctx, run); err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING", Plan: &domain.Plan{}}
	repo, meta, _ := s.Paths(c)
	passed, err := s.tests(ctx, c.ID, repo, meta, &r)
	if passed || err == nil || f.probes != 3 || len(f.commands) != 0 || f.contributors != 0 || r.RecoveryAttempts != 2 {
		t.Fatalf("repeated expensive work: %+v probes %d commands %d %v", r, f.probes, len(f.commands), err)
	}
	runs, _ := s.Store.Tests(ctx, c.ID)
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatal("probe was counted as a repository test", runs)
	}
}
