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

func TestReactorClassifiesEvidenceWithoutSpendingCodeFixes(t *testing.T) {
	cases := []struct {
		output, kind     string
		network, recover bool
	}{
		{"go: downloading go1.27.1\nproxyconnect tcp: dial tcp 127.0.0.1:9: connectex: actively refused", "network_transport", true, true},
		{"proxyconnect tcp: dial tcp 127.0.0.1:9: connection refused", "network_transport", false, false},
		{"cannot connect to the Docker daemon", "runtime_unavailable", true, false},
		{"WSL access is denied", "runtime_unavailable", true, false},
		{"go: downloading go1.99.1\nreading golang.org/toolchain: 404 Not Found", "dependency_unavailable", true, false},
		{"java.io.FileNotFoundException: wrapper.zip.lck (Access is denied)", "workspace_permissions", true, false},
		{"--- FAIL: TestAdd: wanted 5, got -1", "code_failure", false, true},
	}
	for _, tc := range cases {
		d := diagnoseCommand(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, tc.output, errors.New("exit 1"), tc.network)
		if d.Kind != tc.kind || d.CanRecover != tc.recover {
			t.Fatalf("wrong reaction for %q: %+v", tc.output, d)
		}
	}
}

type reactorRunner struct {
	fixtureRunner
	commands  []domain.VerificationCommand
	outputs   []string
	falseZero bool
}

func (f *reactorRunner) Command(_ context.Context, dir string, cmd domain.VerificationCommand, _ bool) (domain.TestRun, error) {
	f.commands = append(f.commands, cmd)
	output := ""
	if len(f.outputs) > 0 {
		output, f.outputs = f.outputs[0], f.outputs[1:]
	}
	run := domain.TestRun{Command: cmd, Directory: dir, StartedAt: time.Now(), FinishedAt: time.Now(), Output: output}
	if output != "" {
		if !f.falseZero {
			run.ExitCode = 1
		}
		return run, errors.New("fixture command failed")
	}
	return run, nil
}

func TestReactorRetriesApprovedDependencyFailureAndRetainsBothAttempts(t *testing.T) {
	runner := &reactorRunner{outputs: []string{"go: download: TLS handshake timeout", ""}}
	s, c := buildFixture(t, runner)
	repo, meta, _ := s.Paths(c)
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING", Network: true, Plan: &domain.Plan{}}
	passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
	if !passed || err != nil || r.RecoveryAttempts != 1 || r.Incident != nil {
		t.Fatalf("recovery failed: %+v, %v", r, err)
	}
	tests, _ := s.Store.Tests(context.Background(), c.ID)
	if len(tests) != 3 || tests[0].ExitCode == 0 || tests[1].ExitCode != 0 || tests[1].RecoveryOf != tests[0].ID || tests[0].FailureKind != "dependency_network" {
		t.Fatalf("lost failure/recovery evidence: %+v", tests)
	}
	artifact, err := os.ReadFile(filepath.Join(filepath.Dir(repo), "artifacts", "test-output.txt"))
	if err != nil || !strings.Contains(string(artifact), tests[0].ID) || !strings.Contains(string(artifact), tests[1].ID) {
		t.Fatal("recovery artifact lost an attempt", err)
	}
}

func TestReactorStopsSetupFailuresWithoutCodeRetryOrApprovalEscalation(t *testing.T) {
	for _, network := range []bool{false, true} {
		t.Run(map[bool]string{false: "unapproved", true: "persistent"}[network], func(t *testing.T) {
			runner := &reactorRunner{outputs: []string{"proxyconnect tcp: dial tcp 127.0.0.1:9: connection refused", "proxyconnect tcp: dial tcp 127.0.0.1:9: connection refused"}}
			s, c := buildFixture(t, runner)
			repo, meta, _ := s.Paths(c)
			r := domain.ExecutionRecord{ContributionID: c.ID, Network: network, Plan: &domain.Plan{}}
			passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
			expected := 1
			if network {
				expected = 2
			}
			if passed || err == nil || len(runner.commands) != expected || r.FixIterations != 0 || r.Incident == nil || r.Incident.Kind != "network_transport" {
				t.Fatalf("incorrect setup recovery: %+v, %v, calls=%d", r, err, len(runner.commands))
			}
		})
	}
}

func TestReactorInstallsOnlyLockedMissingNPMDependencies(t *testing.T) {
	runner := &reactorRunner{outputs: []string{"Error: Cannot find module 'vitest'", "", ""}}
	s, c := buildFixture(t, runner)
	repo, meta, _ := s.Paths(c)
	if err := os.Remove(filepath.Join(repo, "go.mod")); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"package.json": `{"scripts":{"test":"vitest run"}}`, "package-lock.json": `{"lockfileVersion":3}`} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r := domain.ExecutionRecord{ContributionID: c.ID, Network: true, Plan: &domain.Plan{}}
	passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
	if !passed || err != nil || len(runner.commands) != 3 || strings.Join(runner.commands[1].Arguments, " ") != "ci --ignore-scripts --no-audit --no-fund" || strings.Join(runner.commands[2].Arguments, " ") != "run test" {
		t.Fatalf("locked dependency recovery failed: %+v, %v", runner.commands, err)
	}
}

func TestReactorDoesNotCountRunnerErrorAsPassed(t *testing.T) {
	runner := &reactorRunner{outputs: []string{"verification service failed"}, falseZero: true}
	s, c := buildFixture(t, runner)
	repo, meta, _ := s.Paths(c)
	r := domain.ExecutionRecord{ContributionID: c.ID, Plan: &domain.Plan{}}
	passed, _ := s.tests(context.Background(), c.ID, repo, meta, &r)
	tests, _ := s.Store.Tests(context.Background(), c.ID)
	if passed || tests[0].ExitCode == 0 {
		t.Fatal("runner error fabricated a pass")
	}
}

func TestDeclaredGoTimeoutAndRecoveryBudget(t *testing.T) {
	if d := verificationDeadline(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-timeout", "20m"}}); d != 22*time.Minute {
		t.Fatal(d)
	}
	if d := verificationDeadline(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-timeout=1h"}}); d != 30*time.Minute {
		t.Fatal(d)
	}
	runner := &reactorRunner{outputs: []string{"go: download: TLS handshake timeout"}}
	s, c := buildFixture(t, runner)
	repo, meta, _ := s.Paths(c)
	r := domain.ExecutionRecord{ContributionID: c.ID, Network: true, Plan: &domain.Plan{}, RecoveryAttempts: maxRecoveryActions}
	passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
	if passed || err == nil || len(runner.commands) != 1 || r.RecoveryAttempts != maxRecoveryActions {
		t.Fatal("recovery budget was bypassed")
	}
}

func TestDiagnosisDoesNotReplaceCurrentScopeBlockWithOldNetworkFailure(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	ctx := context.Background()
	if err := s.Store.SaveTest(ctx, domain.TestRun{ID: "old", ContributionID: c.ID, ExitCode: 1, Output: "proxyconnect tcp: 127.0.0.1:9", FinishedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.SaveExecution(ctx, domain.ExecutionRecord{ContributionID: c.ID, Status: "BLOCKED", Summary: "issue scope changed since approval", Network: true}, "fixture", "scope change"); err != nil {
		t.Fatal(err)
	}
	d, err := s.Diagnose(ctx, c.ID)
	if err != nil || d.CanRecover || d.Kind != "execution_boundary" {
		t.Fatalf("old error overrode current status: %+v, %v", d, err)
	}
}

func TestVerificationBlockedAgentUsesRealTestsAndIndependentReview(t *testing.T) {
	runner := &fixtureRunner{blockedVerification: true}
	s, c := buildFixture(t, runner)
	ctx := context.Background()
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, err := s.Store.Execution(ctx, c.ID)
	if err != nil || r.Status != "READY" || r.Incident != nil || r.Review == nil || r.Review.Verdict != "APPROVE" || runner.reviews != 2 {
		t.Fatalf("blocked agent did not pass through real verification and independent review: %+v %v", r, err)
	}
	runs, err := s.Store.Tests(ctx, c.ID)
	if err != nil || len(runs) < 3 || runs[0].ExitCode == 0 || runs[len(runs)-1].ExitCode != 0 {
		t.Fatalf("real failed and passing commands were not retained: %+v %v", runs, err)
	}
	current, _ := s.Store.Contribution(ctx, c.ID)
	if current.State != "READY" || r.PRURL != "" {
		t.Fatal("recovery bypassed human PR approval")
	}
}

func TestBlockedAgentCannotBypassSubstantiveBlockOrMissingPatch(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	repo, _, _ := s.Paths(c)
	if err := s.prepareRuntime(repo); err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID}
	for _, output := range []string{
		`{"status":"BLOCKED","summary":"Requirements are unclear; tests cannot establish the intended behavior."}`,
		`{"status":"BLOCKED","summary":"Fix not implemented; the tests require missing requirements."}`,
		`{"status":"BLOCKED","summary":"Testing stalled on downloads."}`,
	} {
		if err := s.acceptImplementation(context.Background(), c, &r, repo, output); err == nil {
			t.Fatal("substantive blocker or absent patch accepted", output)
		}
	}
}

func TestDependencyInstallationRequiresApprovalAndExistingLock(t *testing.T) {
	for _, network := range []bool{false, true} {
		runner := &reactorRunner{outputs: []string{"Cannot find module 'vitest'"}}
		s, c := buildFixture(t, runner)
		repo, meta, _ := s.Paths(c)
		_ = os.Remove(filepath.Join(repo, "go.mod"))
		if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"scripts":{"test":"vitest run"}}`), 0600); err != nil {
			t.Fatal(err)
		}
		if !network {
			if err := os.WriteFile(filepath.Join(repo, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0600); err != nil {
				t.Fatal(err)
			}
		}
		r := domain.ExecutionRecord{ContributionID: c.ID, Network: network, Plan: &domain.Plan{}}
		passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
		if passed || err == nil || len(runner.commands) != 1 || r.RecoveryAttempts != 0 {
			t.Fatalf("installation bypassed approval or lock requirement: network=%v commands=%+v err=%v", network, runner.commands, err)
		}
	}
}

func TestRecoveryBackoffIsCancelledBeforeAnotherCommand(t *testing.T) {
	runner := &reactorRunner{outputs: []string{"TLS handshake timeout"}}
	s, c := buildFixture(t, runner)
	repo, meta, _ := s.Paths(c)
	r := domain.ExecutionRecord{ContributionID: c.ID, Network: true, Plan: &domain.Plan{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(200*time.Millisecond, cancel)
	defer timer.Stop()
	passed, err := s.tests(ctx, c.ID, repo, meta, &r)
	if passed || !errors.Is(err, context.Canceled) || len(runner.commands) != 1 {
		t.Fatalf("cancelled recovery launched another command: %v, %v, %+v", passed, err, runner.commands)
	}
}
