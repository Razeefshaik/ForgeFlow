package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"time"
)

func (s *Store) Approval(ctx context.Context, id string) (domain.WorkspaceApproval, error) {
	var a domain.WorkspaceApproval
	var b string
	err := s.db.QueryRowContext(ctx, "SELECT data FROM workspace_approvals WHERE contribution_id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal([]byte(b), &a)
	}
	return a, err
}
func (s *Store) Execution(ctx context.Context, id string) (domain.ExecutionRecord, error) {
	var v domain.ExecutionRecord
	var b string
	err := s.db.QueryRowContext(ctx, "SELECT data FROM executions WHERE contribution_id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal([]byte(b), &v)
	}
	return v, err
}
func (s *Store) SaveExecution(ctx context.Context, v domain.ExecutionRecord, actor, message string) error {
	v.UpdatedAt = time.Now().UTC()
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO executions VALUES (?,?) ON CONFLICT(contribution_id) DO UPDATE SET data=excluded.data", v.ContributionID, string(b)); err != nil {
			return err
		}
		return s.event(ctx, tx, "ExecutionUpdated", v.ContributionID, actor, message, v)
	})
}
func (s *Store) SaveAgent(ctx context.Context, v domain.AgentRun) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO agent_runs VALUES (?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data", v.ID, v.ContributionID, string(b)); err != nil {
			return err
		}
		return s.event(ctx, tx, "AgentRunUpdated", v.ContributionID, v.Role, v.Role+": "+v.Status, v)
	})
}
func (s *Store) SaveTest(ctx context.Context, v domain.TestRun) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO test_runs VALUES (?,?,?)", v.ID, v.ContributionID, string(b)); err != nil {
			return err
		}
		return s.event(ctx, tx, "TestRunFinished", v.ContributionID, "test-runner", "Verification command completed", v)
	})
}
func (s *Store) Agents(ctx context.Context) ([]domain.AgentRun, error) {
	return listJSON[domain.AgentRun](ctx, s.db, "SELECT data FROM agent_runs ORDER BY rowid DESC LIMIT 200")
}
func (s *Store) Tests(ctx context.Context, id string) ([]domain.TestRun, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT data FROM test_runs WHERE contribution_id=? ORDER BY rowid", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := []domain.TestRun{}
	for rows.Next() {
		var b string
		var v domain.TestRun
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(b), &v); err != nil {
			return nil, err
		}
		all = append(all, v)
	}
	return all, rows.Err()
}
func (s *Store) RecoverExecutions(ctx context.Context) error {
	cs, err := s.Contributions(ctx)
	if err != nil {
		return err
	}
	for _, c := range cs {
		v, e := s.Execution(ctx, c.ID)
		if errors.Is(e, ErrNotFound) {
			continue
		}
		if e != nil {
			return e
		}
		if v.Status == "RUNNING" {
			v.Status = "BLOCKED"
			v.Summary = "Execution interrupted by server restart. Resume explicitly after inspecting saved outputs."
			state := c.State
			switch state {
			case "READY", "PR_PREPARED", "PR_OPENED", "ABANDONED", "FAILED":
				v.Status = state
			default:
				if state != "BLOCKED" && state != "PAUSED" {
					state = "BLOCKED"
				}
			}
			if e = s.SaveExecutionCheckpoint(ctx, v, state, "system", v.Summary); e != nil {
				return e
			}
		}
	}
	agents, err := listJSON[domain.AgentRun](ctx, s.db, "SELECT data FROM agent_runs WHERE json_extract(data,'$.status')='RUNNING'")
	if err != nil {
		return err
	}
	for _, a := range agents {
		if a.Status == "RUNNING" {
			end := time.Now().UTC()
			a.Status = "INTERRUPTED"
			a.FinishedAt = &end
			if err = s.SaveAgent(ctx, a); err != nil {
				return err
			}
		}
	}
	return nil
}
