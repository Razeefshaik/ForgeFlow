package execution

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"forgeflow/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecoveryClassifiesSetupAndActualAssertions(t *testing.T) {
	for _, tc := range []struct {
		output, kind string
		retry        bool
	}{
		{"go: creating work dir: mkdir S:/ForgeFlow/.cache/runtime-tmp/go-build123: Access is denied.", "workspace_runtime", true},
		{"could not read username for 'https://github.com': terminal prompts disabled", "authentication", false},
		{"no space left on device", "disk_capacity", false},
		{"environment image pull failed: failed to connect to the Docker API at npipe:////./pipe/dockerDesktopLinuxEngine", "runtime_unavailable", false},
		{"Traceback:\nFile \"/usr/lib/python3.14/subprocess.py\"\nFileNotFoundError: [Errno 2] No such file or directory: 'semgrep'", "tool_unavailable", false},
		{"FileNotFoundError: [Errno 2] No such file or directory: 'source.txt'", "code_failure", false},
		{"subprocess.TimeoutExpired: Command '['semgrep', 'scan']' timed out after 20 seconds", "verification_timeout", false},
		{"Error: spawnSync go ETIMEDOUT", "verification_timeout", false},
		{"Development MCP failed: Command '['git', 'status', '--porcelain']' timed out after 10 seconds", "verification_timeout", false},
		{"file is being used by another process", "resource_busy", true},
		{"fatal: detected dubious ownership in repository at '/workspace'", "environment_blocked", false},
		{"--- FAIL: TestGit: unexpected fatal: detected dubious ownership in repository", "code_failure", false},
		{"--- FAIL: TestPermissions: expected permission denied, got nil", "code_failure", false},
		{"Input exceeds the maximum length of 1048576 characters", "input_too_large", false},
		{"Codex session failed: HTTP 503 service unavailable", "dependency_network", true},
	} {
		d := diagnoseCommand(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, tc.output, errors.New("exit 1"), true)
		if d.Kind != tc.kind || d.Retryable != tc.retry {
			t.Fatalf("%q: %+v", tc.output, d)
		}
	}
}

func TestWorkspaceRuntimeRecoveryRetriesOriginalCommandWithoutCodeFix(t *testing.T) {
	runner := &reactorRunner{outputs: []string{"go: creating work dir: mkdir outside/go-build123: Access is denied.", ""}}
	s, c := buildFixture(t, runner)
	repo, meta, _ := s.Paths(c)
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING", Plan: &domain.Plan{}}
	passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
	if !passed || err != nil || r.RecoveryAttempts != 1 || r.FixIterations != 0 || runner.contributors != 0 {
		t.Fatalf("runtime recovery: %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".forgeflow-runtime", "tmp")); err != nil {
		t.Fatal(err)
	}
	runs, _ := s.Store.Tests(context.Background(), c.ID)
	if len(runs) != 3 || runs[0].FailureKind != "workspace_runtime" || runs[1].RecoveryOf != runs[0].ID {
		t.Fatalf("lost original failure: %+v", runs)
	}
}

func TestAuthenticationPausesForInputBeforeSpendingCodeFixes(t *testing.T) {
	runner := &reactorRunner{outputs: []string{"401 Unauthorized: authentication required"}}
	s, c := buildFixture(t, runner)
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
	current, _ := s.Store.Contribution(ctx, c.ID)
	if saved.Status != "NEEDS_INPUT" || current.State != "PAUSED" || current.PreviousState != "TESTING" || saved.Incident == nil || saved.Incident.Kind != "authentication" || saved.FixIterations != 0 || runner.contributors != 0 || runner.reviews != 0 || len(runner.commands) != 1 {
		t.Fatalf("auth failure retried coding: %+v %+v", saved, current)
	}
}

type plannerRecoveryRunner struct {
	fixtureRunner
	responses []string
	prompts   []string
}

func (f *plannerRecoveryRunner) Run(ctx context.Context, request codex.Request, onEvent func(json.RawMessage) error) (codex.Result, error) {
	if request.Role != "planner" {
		return f.fixtureRunner.Run(ctx, request, onEvent)
	}
	f.prompts = append(f.prompts, request.Prompt)
	if len(f.responses) == 0 {
		return codex.Result{}, errors.New("unexpected planner call")
	}
	output := f.responses[0]
	f.responses = f.responses[1:]
	return codex.Result{Output: output}, nil
}
func TestInvalidPlanReceivesFeedbackBeforeAnyCodeOrTests(t *testing.T) {
	runner := &plannerRecoveryRunner{responses: []string{`{"summary":"Verify","tests":[{"program":"go","arguments":["test","|","echo"]}]}`, `{"summary":"Verify","tests":[{"program":"go","arguments":["test","-run","TestOne|TestTwo","./..."]}]}`}}
	s, c := buildFixture(t, runner)
	repo, meta, _ := s.Paths(c)
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING", Phase: "PLANNING"}
	plan, err := s.planning(context.Background(), c, &r, repo, meta, "Approved constraints\n")
	if err != nil || plan.Summary != "Verify" || len(runner.prompts) != 2 || !strings.Contains(runner.prompts[1], "argument 1") || r.RecoveryAttempts != 1 || runner.contributors != 0 {
		t.Fatalf("planner correction: %+v %v", plan, err)
	}
	files, _ := filepath.Glob(filepath.Join(meta, "rejected-plan-*.json"))
	if len(files) != 1 {
		t.Fatal("rejected evidence was lost")
	}
	if _, err := os.Stat(filepath.Join(meta, "plan.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("planning saved an unapproved plan")
	}
	runs, _ := s.Store.Tests(context.Background(), c.ID)
	if len(runs) != 0 {
		t.Fatal("tests ran before a validated plan")
	}
}

func TestInvalidPlanningStopsAtBoundedLimit(t *testing.T) {
	runner := &plannerRecoveryRunner{responses: []string{"not JSON", "not JSON", "not JSON"}}
	s, c := buildFixture(t, runner)
	repo, meta, _ := s.Paths(c)
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING"}
	_, err := s.planning(context.Background(), c, &r, repo, meta, "")
	if err == nil || len(runner.prompts) != 3 || r.RecoveryAttempts != 2 || r.Plan != nil || runner.contributors != 0 {
		t.Fatalf("unbounded planner retries: %+v %v", r, err)
	}
}

func TestLegacyRejectedRegexRecoversThroughPlanApproval(t *testing.T) {
	runner := &fixtureRunner{}
	s, c := buildFixtureWithApproval(t, runner, "", true)
	ctx := context.Background()
	output := `{"summary":"Inspect self-target guard","tests":[{"program":"go","arguments":["test","-run","TestOne|TestTwo|TestThree","./..."]}]}`
	if err := s.Store.SaveAgent(ctx, domain.AgentRun{ID: storage.ID(), ContributionID: c.ID, Role: "planner", Status: "SUCCEEDED", Output: output}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.SaveExecution(ctx, domain.ExecutionRecord{ContributionID: c.ID, Status: "BLOCKED", Phase: "PLANNING", PlanApproved: true, Summary: "unsafe verification argument", Incident: &domain.ExecutionIncident{Kind: "execution_blocked", CanRecover: false}}, "fixture", "old rejection"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recover(ctx, c.ID, false, false); err == nil {
		t.Fatal("recovery bypassed approval")
	}
	if _, err := s.Recover(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, _ := s.Store.Execution(ctx, c.ID)
	if r.Status != "AWAITING_PLAN_APPROVAL" || r.Plan == nil || r.PlanApproved || runner.contributors != 0 || len(runner.roles) != 0 {
		t.Fatalf("legacy plan bypassed approval or discarded valid evidence: %+v", r)
	}
}

func TestRateLimitAndOperationRecoveryAreBounded(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING"}
	calls := 0
	err := s.recoverOperation(context.Background(), &r, "GitHub freshness", func() error {
		calls++
		if calls == 1 {
			return &github.APIError{Status: 429, RetryAt: time.Now().Add(10 * time.Millisecond)}
		}
		return nil
	})
	if err != nil || calls != 2 || r.RecoveryAttempts != 1 || r.Incident != nil {
		t.Fatalf("short rate-limit recovery: %+v %v", r, err)
	}
	r = domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING"}
	calls = 0
	err = s.recoverOperation(context.Background(), &r, "GitHub freshness", func() error { calls++; return &github.APIError{Status: 429, RetryAt: time.Now().Add(time.Hour)} })
	if err == nil || calls != 1 || r.RecoveryAttempts != 0 || r.Incident == nil || r.Incident.RetryAt == nil {
		t.Fatal("long rate limit occupied a running retry loop")
	}
}

type partialAgentRunner struct {
	fixtureRunner
	attempts  int
	preserved bool
}

func (f *partialAgentRunner) Run(_ context.Context, request codex.Request, _ func(json.RawMessage) error) (codex.Result, error) {
	f.attempts++
	file := filepath.Join(request.Directory, "partial-patch.txt")
	if f.attempts == 1 {
		if err := os.WriteFile(file, []byte("keep this work"), 0600); err != nil {
			return codex.Result{}, err
		}
		return codex.Result{}, errors.New("Codex session failed: HTTP 503 Service Unavailable")
	}
	data, err := os.ReadFile(file)
	f.preserved = err == nil && string(data) == "keep this work"
	return codex.Result{Output: `{"status":"IMPLEMENTED","summary":"Partial work preserved"}`}, nil
}

func TestCodexTransportRecoveryPreservesPartialWorkAndErrorEvidence(t *testing.T) {
	runner := &partialAgentRunner{}
	s, c := buildFixture(t, runner)
	repo, meta, _ := s.Paths(c)
	if err := s.prepareRuntime(repo); err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING", Phase: "CODING"}
	_, err := s.recoveringAgent(context.Background(), c, &r, repo, meta, "contributor", "Implement the approved plan", implementationSchema)
	if err != nil || runner.attempts != 2 || !runner.preserved || r.RecoveryAttempts != 1 {
		t.Fatalf("partial work recovery: %+v %v", r, err)
	}
	agents, err := s.Store.Agents(context.Background())
	if err != nil || len(agents) != 2 || agents[1].Status != "FAILED" || !strings.Contains(agents[1].Error, "HTTP 503") {
		t.Fatalf("lost transport failure evidence: %+v %v", agents, err)
	}
}

func TestRecoverCodingConnectionFailureRetainsPhaseAndPlanGate(t *testing.T) {
	runner := &fixtureRunner{}
	s, c := buildFixtureWithApproval(t, runner, "", true)
	ctx := context.Background()
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "NEEDS_INPUT", Phase: "CODING", Plan: &domain.Plan{Summary: "Saved plan"}, Incident: &domain.ExecutionIncident{Kind: "dependency_network", Source: "codex", CanRecover: true}}
	if err := s.Store.SaveExecutionCheckpoints(ctx, r, []string{"PLANNING", "CODING", "PAUSED"}, "fixture", "connection failure"); err != nil {
		t.Fatal(err)
	}
	started, err := s.Recover(ctx, c.ID, true, false)
	if err != nil || started.Phase != "CODING" || started.Network {
		t.Fatalf("coding recovery skipped phase or escalated network: %+v %v", started, err)
	}
	s.Wait()
	saved, _ := s.Store.Execution(ctx, c.ID)
	if saved.Status != "AWAITING_PLAN_APPROVAL" || runner.contributors != 0 || runner.reviews != 0 {
		t.Fatalf("recovery bypassed plan gate: %+v", saved)
	}
	runs, _ := s.Store.Tests(ctx, c.ID)
	if len(runs) != 0 {
		t.Fatal("incomplete implementation jumped to tests")
	}
}

func TestCancelledOperationDoesNotLaunchOrRetry(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	r := domain.ExecutionRecord{ContributionID: c.ID}
	err := s.recoverOperation(ctx, &r, "GitHub freshness", func() error { calls++; return nil })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("cancelled operation was launched", err)
	}
}

func TestRecoveryDiagnosisIncludesBoundedObservedFailure(t *testing.T) {
	output := "go: creating work dir: mkdir S:/old-project/runtime: Access is denied.\n" + strings.Repeat("detail\n", 5000)
	d := diagnoseCommand(domain.VerificationCommand{Program: "go"}, output, errors.New("exit status 1"), false)
	if !strings.Contains(d.Evidence, "S:/old-project/runtime") || !strings.Contains(d.Evidence, "exit status 1") || len(d.Evidence) > 3000 {
		t.Fatal("diagnosis lost actual cause or exceeded inline evidence bound")
	}
}

func TestNewConstraintsForceFreshPlanningInsteadOfLegacyReuse(t *testing.T) {
	runner := &plannerRecoveryRunner{responses: []string{`{"summary":"Respect new scope","tests":[]}`}}
	s, c := buildFixture(t, runner)
	ctx := context.Background()
	if err := s.Store.SaveAgent(ctx, domain.AgentRun{ID: storage.ID(), ContributionID: c.ID, Role: "planner", Status: "SUCCEEDED", Output: `{"summary":"Old scope","tests":[]}`}); err != nil {
		t.Fatal(err)
	}
	repo, meta, _ := s.Paths(c)
	r := domain.ExecutionRecord{ContributionID: c.ID, Constraints: "Keep the public API unchanged"}
	plan, err := s.planning(ctx, c, &r, repo, meta, r.Constraints)
	if err != nil || plan.Summary != "Respect new scope" || len(runner.prompts) != 1 || !strings.Contains(runner.prompts[0], r.Constraints) {
		t.Fatal("fresh constraints reused a stale plan", err)
	}
}
