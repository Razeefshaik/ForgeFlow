package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"testing"
	"time"
)

func TestContainerInfrastructureDoesNotTriggerSourceFix(t *testing.T) {
	for _, message := range []string{"container creation failed: no such image", "container cleanup failed for forgeflow-verify-fixture", "container isolation check failed: root write succeeded"} {
		incident := diagnoseCommand(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, "", errors.New(message), false)
		if incident.Kind != "runtime_unavailable" || incident.CanRecover || incident.Retryable {
			t.Fatal(incident)
		}
	}
}

type preflightDeadlineRunner struct {
	fixtureRunner
	remaining time.Duration
}

func (*preflightDeadlineRunner) CheckTimeout() time.Duration { return 225 * time.Second }
func (f *preflightDeadlineRunner) Check(ctx context.Context, _, _ string) error {
	deadline, _ := ctx.Deadline()
	f.remaining = time.Until(deadline)
	return errors.New("container isolation check failed: deliberate fixture stop")
}

func TestPreflightHonorsContainerStartupBudgetBeforeAnyAgent(t *testing.T) {
	runner := &preflightDeadlineRunner{}
	s, c := buildFixture(t, runner)
	if _, err := s.Start(context.Background(), c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if runner.remaining < 220*time.Second || runner.remaining > 225*time.Second || len(runner.roles) != 0 {
		t.Fatalf("preflight budget=%s; agents=%v", runner.remaining, runner.roles)
	}
	r, err := s.Store.Execution(context.Background(), c.ID)
	if err != nil || r.Status != "NEEDS_INPUT" || r.FixIterations != 0 {
		t.Fatal("preflight failure spent a code fix", r, err)
	}
}
