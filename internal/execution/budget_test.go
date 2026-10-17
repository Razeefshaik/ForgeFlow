package execution

import (
	"context"
	"strings"
	"testing"
)

// A stop at the fix limit is surfaced to the human; their explicit resume
// must be able to continue instead of hitting the same limit immediately.
func TestHumanResumeRenewsExhaustedFixBudget(t *testing.T) {
	runner := &fixtureRunner{alwaysFail: true}
	s, c := buildFixture(t, runner)
	ctx := context.Background()
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, _ := s.Store.Execution(ctx, c.ID)
	if !strings.Contains(r.Summary, "maximum fix") {
		t.Fatalf("fixture did not exhaust its budget: %+v", r)
	}
	// The human inspects the evidence and resumes; the environment problem
	// behind the repeated failure has been fixed meanwhile.
	runner.alwaysFail = false
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, _ = s.Store.Execution(ctx, c.ID)
	if r.Status != "READY" {
		t.Fatalf("resume after the limit could not continue: %+v", r)
	}
	renewed := false
	for cursor := int64(0); ; {
		events, err := s.Store.Events(ctx, cursor, c.ID, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range events {
			renewed = renewed || e.Type == "ExecutionBudgetRenewed"
			cursor = e.ID
		}
		if len(events) < 100 {
			break
		}
	}
	if !renewed {
		t.Fatal("budget renewal was not audited")
	}
}
