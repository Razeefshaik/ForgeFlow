package execution

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"strings"
	"testing"
)

type environmentRunner struct {
	fixtureRunner
	failPreparation bool
	requestRepair   bool
	planners        int
	preparations    int
	lastPrompt      string
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
	} {
		d := diagnoseCommand(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, output, errors.New("exit status 1"), true)
		if d.Kind != "toolchain_mismatch" || d.CanRecover || d.Retryable || !inputRequired(d.Kind) {
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
