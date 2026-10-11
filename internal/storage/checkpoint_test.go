package storage

import (
	"context"
	"database/sql/driver"
	"fmt"
	"forgeflow/internal/domain"
	"path/filepath"
	"testing"
)

func checkpointFixture(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), ":memory:", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	err = s.SeedOnce(context.Background(), []domain.Opportunity{{ID: "o", Demo: true}}, []domain.Contribution{{ID: "c", OpportunityID: "o", State: "CODING", Demo: true}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCheckpointRollsBackWholeTransitionChain(t *testing.T) {
	s := checkpointFixture(t)
	ctx := context.Background()
	r := domain.ExecutionRecord{ContributionID: "c", Status: "RUNNING", Phase: "TESTING"}
	before, _ := s.Events(ctx, 0, "", 100)
	if err := s.SaveExecutionCheckpoints(ctx, r, []string{"TESTING", "PR_OPENED"}, "fixture", "invalid"); err == nil {
		t.Fatal("invalid chain committed")
	}
	c, _ := s.Contribution(ctx, "c")
	after, _ := s.Events(ctx, 0, "", 100)
	if c.State != "CODING" || len(after) != len(before) {
		t.Fatal("partial state or events persisted")
	}
	if _, err := s.Execution(ctx, "c"); err != ErrNotFound {
		t.Fatal("execution persisted on rollback", err)
	}
	if err := s.SaveExecutionCheckpoint(ctx, r, "TESTING", "fixture", "valid"); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Contribution(ctx, "c")
	saved, _ := s.Execution(ctx, "c")
	if c.State != saved.Phase {
		t.Fatal("state split")
	}
}
func TestRestartRecoveryIncludesAgentsBeyondDisplayLimit(t *testing.T) {
	s := checkpointFixture(t)
	ctx := context.Background()
	for i := 0; i < 205; i++ {
		status := "SUCCEEDED"
		if i == 0 {
			status = "RUNNING"
		}
		if err := s.SaveAgent(ctx, domain.AgentRun{ID: fmt.Sprint(i), ContributionID: "c", Status: status}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveExecution(ctx, domain.ExecutionRecord{ContributionID: "c", Status: "RUNNING", Phase: "CODING"}, "fixture", "running"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverExecutions(ctx); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, "SELECT json_extract(data,'$.status') FROM agent_runs WHERE id='0'").Scan(&raw); err != nil || raw != "INTERRUPTED" {
		t.Fatal("old running agent ignored", raw, err)
	}
	c, _ := s.Contribution(ctx, "c")
	r, _ := s.Execution(ctx, "c")
	if c.State != "BLOCKED" || r.Status != "BLOCKED" || c.PreviousState != "CODING" {
		t.Fatal("restart state inconsistent")
	}
}

func TestReplacementConnectionRetainsSQLiteSafety(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "reconnect.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.Raw(func(any) error { return driver.ErrBadConn }); err != driver.ErrBadConn {
		t.Fatal(err)
	}
	conn.Close()
	for pragma, want := range map[string]int{"foreign_keys": 1, "busy_timeout": 5000} {
		var got int
		if err = s.db.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&got); err != nil || got != want {
			t.Fatalf("replacement connection %s=%d want %d: %v", pragma, got, want, err)
		}
	}
}

func TestRestartRecoveryRespectsPausedAndCompletedStates(t *testing.T) {
	for _, state := range []string{"PAUSED", "READY"} {
		t.Run(state, func(t *testing.T) {
			s := checkpointFixture(t)
			ctx := context.Background()
			states := []string{state}
			if state == "READY" {
				states = []string{"TESTING", "REVIEWING", "READY"}
			}
			r := domain.ExecutionRecord{ContributionID: "c", Status: "RUNNING", Phase: "CODING", Summary: "Implemented addition", Message: "Saved work"}
			if err := s.SaveExecutionCheckpoints(ctx, r, states, "fixture", "legacy mismatch"); err != nil {
				t.Fatal(err)
			}
			if err := s.RecoverExecutions(ctx); err != nil {
				t.Fatal(err)
			}
			saved, _ := s.Execution(ctx, "c")
			c, _ := s.Contribution(ctx, "c")
			if saved.Status != state || c.State != state {
				t.Fatalf("restart mismatch: %s %s", saved.Status, c.State)
			}
			if state == "READY" && saved.Summary != r.Summary {
				t.Fatal("completed implementation summary overwritten")
			}
		})
	}
}
