package execution

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type environmentRunner struct {
	fixtureRunner
	failPreparation bool
	requestRepair   bool
	planners        int
	preparations    int
	lastPrompt      string
}

type inventoryApprovalRunner struct {
	environmentRunner
	sawApproval bool
}

type unavailableDockerRunner struct{ environmentRunner }

func (f *unavailableDockerRunner) PrepareEnvironment(context.Context, string, bool, domain.EnvironmentPlan) (domain.ExecutionEnvironment, error) {
	f.preparations++
	err := errors.New("local image unavailable: failed to connect to the Docker API at npipe:////./pipe/dockerDesktopLinuxEngine")
	return domain.ExecutionEnvironment{Status: "blocked", Error: err.Error(), Fingerprint: "same-inputs"}, err
}

func TestDockerOutageDoesNotSpendAnotherEnvironmentPlan(t *testing.T) {
	f := &unavailableDockerRunner{}
	s, c := buildFixture(t, f)
	if _, err := s.Start(context.Background(), c.ID, true, true); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, err := s.Store.Execution(context.Background(), c.ID)
	if err != nil || r.Incident == nil || r.Incident.Kind != "runtime_unavailable" || r.Status != "NEEDS_INPUT" || f.planners != 1 || f.preparations != 1 || f.contributors != 0 || r.FixIterations != 0 {
		t.Fatalf("outage wasted model work: %+v; counts=%d/%d/%d; error=%v", r, f.planners, f.preparations, f.contributors, err)
	}
}

func (*inventoryApprovalRunner) InspectEnvironment(context.Context, string) (domain.ExecutionEnvironment, error) {
	return domain.ExecutionEnvironment{Status: "inspected", Image: "fixture-image", Network: false}, nil
}

func (f *inventoryApprovalRunner) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	if r.Role == "environment-planner" {
		paths, err := filepath.Glob(filepath.Join(r.Directory, ".forgeflow-runtime", "evidence", "environment-*.json"))
		if err != nil || len(paths) == 0 {
			return codex.Result{}, errors.New("missing inventory snapshot")
		}
		b, err := os.ReadFile(paths[len(paths)-1])
		if err != nil {
			return codex.Result{}, err
		}
		var evidence struct {
			Approved    bool                        `json:"current_downloads_approved"`
			Environment domain.ExecutionEnvironment `json:"environment"`
		}
		if err = json.Unmarshal(b, &evidence); err != nil {
			return codex.Result{}, err
		}
		f.sawApproval = evidence.Approved && evidence.Environment.Network && strings.Contains(r.Prompt, "Correct Add")
	}
	return f.environmentRunner.Run(ctx, r, event)
}

func TestOfflineInventoryDoesNotRevokeDownloadApproval(t *testing.T) {
	runner := &inventoryApprovalRunner{}
	s, c := buildFixture(t, runner)
	repo, meta, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID, Network: true}
	if err = s.prepareEnvironment(context.Background(), c, &r, repo, meta, false); err != nil {
		t.Fatal(err)
	}
	if !runner.sawApproval {
		t.Fatal("planner saw stale approval or no approved issue")
	}
}

func TestBlockedSavedRecipeRequestsFreshPlanOnResume(t *testing.T) {
	runner := &environmentRunner{failPreparation: true}
	s, c := buildFixture(t, runner)
	repo, meta, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID}
	if err = s.prepareEnvironment(context.Background(), c, &r, repo, meta, false); err == nil {
		t.Fatal("fixture should block")
	}
	if err = s.prepareEnvironment(context.Background(), c, &r, repo, meta, false); err == nil {
		t.Fatal("unchanged repair should stop")
	}
	if runner.planners != 2 || runner.preparations != 1 {
		t.Fatal("replayed blocked setup", runner.planners, runner.preparations)
	}
}

func TestDownloadApprovalAllowsOneUnchangedRecipeReplay(t *testing.T) {
	runner := &environmentRunner{failPreparation: true}
	s, c := buildFixture(t, runner)
	repo, meta, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	r := domain.ExecutionRecord{ContributionID: c.ID}
	if err = s.prepareEnvironment(context.Background(), c, &r, repo, meta, false); err == nil {
		t.Fatal("offline setup should block")
	}
	// Public dependencies become provisionable after explicit approval even if
	// the versioned executable recipe itself did not change.
	r.Network = true
	runner.failPreparation = false
	if err = s.prepareEnvironment(context.Background(), c, &r, repo, meta, false); err != nil {
		t.Fatal("changed approval ignored", err)
	}
	if runner.preparations != 2 || r.Environment.Network != true {
		t.Fatal("new approval not used", runner.preparations)
	}
	if err = s.prepareEnvironment(context.Background(), c, &r, repo, meta, true); err == nil {
		t.Fatal("unchanged approved recipe replayed blindly")
	}
	if runner.preparations != 2 {
		t.Fatal("extra setup despite unchanged recipe and approval")
	}
}

func (f *environmentRunner) EnvironmentBase() string { return "fixture:1" }
func (f *environmentRunner) EnvironmentFingerprint(string, domain.EnvironmentPlan) (string, error) {
	return "same-inputs", nil
}
func (f *environmentRunner) PrepareEnvironment(_ context.Context, _ string, network bool, plan domain.EnvironmentPlan) (domain.ExecutionEnvironment, error) {
	f.preparations++
	r := domain.ExecutionEnvironment{Status: "ready", Image: "fixture-image", Fingerprint: "same-inputs", Network: network, Capabilities: []string{"repair environment"}}
	if f.failPreparation {
		r.Status = "blocked"
		r.Error = "required toolchain unavailable"
		return r, errors.New(r.Error)
	}
	return r, nil
}
func (f *environmentRunner) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	if r.Role == "environment-planner" {
		f.planners++
		p := domain.EnvironmentPlan{Summary: "All repository requirements", BaseImage: "fixture:1", Checks: []domain.VerificationCommand{{Program: "custom-tool", Arguments: []string{"--version"}}}}
		b, _ := json.Marshal(p)
		return codex.Result{Output: string(b)}, nil
	}
	if r.Role == "contributor" {
		f.lastPrompt = r.Prompt
		if f.requestRepair {
			f.contributors++
			return codex.Result{Output: `{"status":"ENVIRONMENT_BLOCKED","summary":"The required private service is unavailable; request environment repair, preserve the patch."}`}, nil
		}
	}
	return f.fixtureRunner.Run(ctx, r, event)
}

func TestEnvironmentFailureStopsBeforeCodingOrTests(t *testing.T) {
	runner := &environmentRunner{failPreparation: true}
	s, c := buildFixture(t, runner)
	if _, err := s.Start(context.Background(), c.ID, true, true); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, err := s.Store.Execution(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	tests, _ := s.Store.Tests(context.Background(), c.ID)
	if r.Status != "NEEDS_INPUT" || r.Incident == nil || r.Incident.Kind != "environment_blocked" || r.FixIterations != 0 || runner.contributors != 0 || len(tests) != 0 || r.Environment == nil {
		t.Fatalf("environment failure triggered coding: %+v; contributors=%d tests=%d", r, runner.contributors, len(tests))
	}
	events, _ := s.Store.Events(context.Background(), 0, c.ID, 100)
	found := false
	for _, event := range events {
		if event.Type == "EnvironmentPreparationFinished" {
			found = true
		}
	}
	if !found {
		t.Fatal("lost provisioning evidence")
	}
}

func TestEnvironmentRequestStopsUnchangedRepairWithoutFixLoop(t *testing.T) {
	runner := &environmentRunner{requestRepair: true}
	s, c := buildFixture(t, runner)
	if _, err := s.Start(context.Background(), c.ID, true, true); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, err := s.Store.Execution(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "NEEDS_INPUT" || runner.contributors != 1 || runner.planners != 2 || r.FixIterations != 0 {
		t.Fatalf("repeated unchanged request: %+v contributors=%d planners=%d", r, runner.contributors, runner.planners)
	}
	if !strings.Contains(runner.lastPrompt, "Control-plane environment evidence") || !strings.Contains(runner.lastPrompt, "ENVIRONMENT_BLOCKED") {
		t.Fatal("contributor did not receive diagnostics/capabilities")
	}
}

func TestToolchainMismatchDoesNotStartSourceFix(t *testing.T) {
	for _, output := range []string{
		"go: go.mod requires go >= 1.27.0 (running go 1.26.1; GOTOOLCHAIN=local)",
		"npm ERR! code EBADENGINE\nnpm ERR! engine Unsupported engine",
		"ERROR: Package requires a different Python: 3.10 not in >=3.12",
		"error: invalid target release: 21",
		"error: package cannot be built because it requires rustc 1.85 or newer",
		"error: rustc 1.85.0 is not supported by the following packages",
		"error NETSDK1045: The current .NET SDK does not support targeting .NET 10.0",
	} {
		d := diagnoseCommand(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, output, errors.New("exit status 1"), true)
		if d.Kind != "toolchain_mismatch" || d.CanRecover || d.Retryable || !inputRequired(d.Kind) {
			t.Fatal(output, d)
		}
	}
}

func TestMissingLibraryIsEnvironmentEvidence(t *testing.T) {
	for _, output := range []string{"ModuleNotFoundError: No module named 'pytest'", "Bundler::GemNotFound: missing rack", "error while loading shared libraries: libssl.so: cannot open shared object file"} {
		d := diagnoseCommand(domain.VerificationCommand{Program: "custom-tool"}, output, errors.New("exit status 1"), true)
		if d.Kind != "dependencies_missing" || d.Retryable {
			t.Fatal(output, d)
		}
	}
}

func TestStructuredEnvironmentBlockPreservesPatchAndIncident(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	r := domain.ExecutionRecord{ContributionID: c.ID, Phase: "CODING"}
	err := s.acceptImplementation(context.Background(), c, &r, c.Workspace, `{"status":"ENVIRONMENT_BLOCKED","summary":"Need the pinned SDK and service fixture."}`)
	if !errors.Is(err, environmentBlocked) || r.Incident == nil || r.Incident.Kind != "environment_blocked" {
		t.Fatal(err, r)
	}
}

func TestCodexUsageLimitDoesNotRetryOrFix(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	r := domain.ExecutionRecord{ContributionID: c.ID}
	calls := 0
	err := s.recoverOperation(context.Background(), &r, "Codex environment-planner", func() error {
		calls++
		return errors.New("Codex session failed: You've hit your usage limit. Try again after reset.")
	})
	if err == nil || calls != 1 || r.Incident == nil || r.Incident.Kind != "usage_limit" || r.Incident.Retryable || r.FixIterations != 0 || !inputRequired(r.Incident.Kind) {
		t.Fatalf("quota triggered retries: calls=%d record=%+v", calls, r)
	}
}

func TestAssertionQuotingToolchainErrorStillNeedsCodeInvestigation(t *testing.T) {
	d := diagnoseCommand(domain.VerificationCommand{Program: "python3"}, "AssertionError: expected diagnostic 'requires a different Python'", errors.New("exit status 1"), true)
	if d.Kind != "code_failure" {
		t.Fatal(d)
	}
}

type verificationEnvironmentRunner struct {
	environmentRunner
	commands []domain.VerificationCommand
	deny     bool
}

func (f *verificationEnvironmentRunner) EnvironmentFingerprint(_ string, p domain.EnvironmentPlan) (string, error) {
	return p.BaseImage, nil
}
func (f *verificationEnvironmentRunner) PrepareEnvironment(_ context.Context, _ string, network bool, p domain.EnvironmentPlan) (domain.ExecutionEnvironment, error) {
	f.preparations++
	return domain.ExecutionEnvironment{Status: "ready", Image: "repaired-image", Fingerprint: p.BaseImage, Network: network}, nil
}
func (f *verificationEnvironmentRunner) Run(_ context.Context, r codex.Request, _ func(json.RawMessage) error) (codex.Result, error) {
	if r.Role != "environment-planner" {
		return codex.Result{}, errors.New("unexpected source agent")
	}
	f.planners++
	p := domain.EnvironmentPlan{Summary: "Match the compiler requirement", BaseImage: "fixture:2", Checks: []domain.VerificationCommand{{Program: "compiler", Arguments: []string{"--version"}}}}
	if f.deny {
		p.BaseImage = "fixture:1"
	}
	b, _ := json.Marshal(p)
	return codex.Result{Output: string(b)}, nil
}
func (f *verificationEnvironmentRunner) Command(_ context.Context, dir string, c domain.VerificationCommand, _ bool) (domain.TestRun, error) {
	f.commands = append(f.commands, c)
	r := domain.TestRun{Command: c, Directory: dir, StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC()}
	if len(f.commands) == 1 {
		r.ExitCode = 1
		r.Output = "go: go.mod requires go >= 1.27.0 (running go 1.26.1; GOTOOLCHAIN=local)"
		return r, errors.New("exit status 1")
	}
	r.Output = "PASS"
	return r, nil
}

func TestVerificationRepairRerunsExactCommandWithoutCodeFix(t *testing.T) {
	for _, unchanged := range []bool{false, true} {
		t.Run(map[bool]string{false: "changed", true: "unchanged"}[unchanged], func(t *testing.T) {
			runner := &verificationEnvironmentRunner{deny: unchanged}
			s, c := buildFixture(t, runner)
			repo, _, err := s.Paths(c)
			if err != nil {
				t.Fatal(err)
			}
			r := domain.ExecutionRecord{ContributionID: c.ID, Phase: "TESTING", Network: true, Environment: &domain.ExecutionEnvironment{Status: "ready", Image: "old-image", Fingerprint: "fixture:1"}, EnvironmentPlan: &domain.EnvironmentPlan{Summary: "previous", BaseImage: "fixture:1", Checks: []domain.VerificationCommand{{Program: "compiler", Arguments: []string{"--version"}}}}}
			command := domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-race", "./..."}}
			var output strings.Builder
			passed, err := s.verifyWithRecovery(context.Background(), c.ID, repo, command, &r, &output)
			if unchanged {
				if err == nil || passed || len(runner.commands) != 1 || runner.preparations != 0 {
					t.Fatal("unchanged repair reran checks", passed, err, runner.commands)
				}
			} else {
				if err != nil || !passed || len(runner.commands) != 2 || runner.preparations != 1 {
					t.Fatal("changed repair failed", passed, err, runner.commands)
				}
				for _, actual := range runner.commands {
					if actual.Program != command.Program || strings.Join(actual.Arguments, " ") != strings.Join(command.Arguments, " ") {
						t.Fatal("weakened verification", actual)
					}
				}
				tests, _ := s.Store.Tests(context.Background(), c.ID)
				if len(tests) != 2 || tests[0].ExitCode == 0 || tests[1].RecoveryOf != tests[0].ID {
					t.Fatal("lost failed/recovery evidence", tests)
				}
			}
			if r.FixIterations != 0 || runner.contributors != 0 {
				t.Fatal("environment repair spent source-fix iteration")
			}
		})
	}
}
