package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const assertionBeforeTimeout = "--- FAIL: TestLSP_Boundary_FallbackLengthMatchesScanner (0.58s)\n    policy_test.go:234: 61 lines: scanner 0, language server 1, want 1\nFAIL\nFAIL example.com/fixture/cmd/lsp 5.213s\n"

func TestPartialAssertionsAndTimeoutAreBothDiagnosed(t *testing.T) {
	command := domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}
	d := diagnoseCommand(command, assertionBeforeTimeout, context.DeadlineExceeded, true)
	if d.Kind != "test_failure_timeout" || len(d.SecondaryKinds) != 2 || len(d.FailedTests) != 1 || !strings.Contains(d.Evidence, "scanner 0") {
		t.Fatal(d)
	}
	d = diagnoseCommand(command, "panic: test timed out after 10m0s\ngoroutine 1", errors.New("exit status 1"), true)
	if d.Kind != "verification_timeout" {
		t.Fatal(d)
	}
}

type investigationRunner struct {
	reactorRunner
	first  bool
	repeat bool
}

func (f *investigationRunner) Command(ctx context.Context, dir string, cmd domain.VerificationCommand, network bool) (domain.TestRun, error) {
	if !f.first || (f.repeat && len(cmd.Arguments) == 2 && cmd.Arguments[0] == "test") {
		f.first = true
		f.commands = append(f.commands, cmd)
		return domain.TestRun{ExitCode: 1, Output: assertionBeforeTimeout}, context.DeadlineExceeded
	}
	return f.reactorRunner.Command(ctx, dir, cmd, network)
}
func TestTimeoutInvestigationNeverReplacesCompleteVerification(t *testing.T) {
	for _, output := range []string{"--- FAIL: TestLSP_Boundary_FallbackLengthMatchesScanner\nassertion failed", "", "couldn't create signal pipe, Win32 error 5"} {
		t.Run(output, func(t *testing.T) {
			f := &investigationRunner{reactorRunner: reactorRunner{outputs: []string{output}}, repeat: output == ""}
			s, c := buildFixture(t, f)
			repo, meta, _ := s.Paths(c)
			if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/fixture\n\ngo 1.23\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(repo, "cmd", "lsp"), 0700); err != nil {
				t.Fatal(err)
			}
			r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING", Phase: "TESTING", Network: true, Plan: &domain.Plan{}}
			passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
			if passed || len(f.commands) < 2 || !strings.Contains(strings.Join(f.commands[1].Arguments, " "), "./cmd/lsp") || f.contributors != 0 {
				t.Fatal(passed, err, f.commands)
			}
			runs, _ := s.Store.Tests(context.Background(), c.ID)
			if runs[1].RecoveryOf != runs[0].ID || runs[0].Error != context.DeadlineExceeded.Error() {
				t.Fatal(runs)
			}
			if output == "" && (err == nil || len(f.commands) != 3 || r.Incident.Status != "needs_input") {
				t.Fatal("focused pass became full pass", r, err)
			}
			if strings.Contains(output, "assertion") && (err != nil || r.Incident.Kind != "test_failure_timeout") {
				t.Fatal("assertion investigation stopped", r, err)
			}
			if strings.Contains(output, "signal pipe") && (err == nil || r.Incident.Kind != "sandbox_process") {
				t.Fatal("setup sent to code fix", r, err)
			}
		})
	}
}

func TestFocusedPassRequiresSuccessfulCompleteSuiteBeforeContinuing(t *testing.T) {
	f := &investigationRunner{}
	s, c := buildFixture(t, f)
	repo, meta, _ := s.Paths(c)
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/fixture\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "cmd", "lsp"), 0700); err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING", Phase: "TESTING", Plan: &domain.Plan{}}
	passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
	if !passed || err != nil || len(f.commands) != 4 || r.Incident != nil {
		t.Fatal(passed, err, f.commands, r)
	}
	runs, _ := s.Store.Tests(context.Background(), c.ID)
	if runs[0].ExitCode == 0 || runs[1].RecoveryOf != runs[0].ID || runs[2].RecoveryOf != runs[0].ID || strings.Join(runs[2].Command.Arguments, " ") != "test ./..." {
		t.Fatal("focused check replaced suite", runs)
	}
}
