package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"time"
)

func (s *Store) Contribution(ctx context.Context, id string) (domain.Contribution, error) {
	var c domain.Contribution
	var body string
	err := s.db.QueryRowContext(ctx, "SELECT data FROM contributions WHERE id=?", id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal([]byte(body), &c)
	return c, err
}
func (s *Store) ApprovedContribution(ctx context.Context, token string) (domain.Contribution, error) {
	var id string
	err := s.db.QueryRowContext(ctx, "SELECT contribution_id FROM workspace_approvals WHERE token=?", token).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Contribution{}, ErrNotFound
	}
	if err != nil {
		return domain.Contribution{}, err
	}
	return s.Contribution(ctx, id)
}
func (s *Store) ApproveWorkspace(ctx context.Context, c domain.Contribution, a domain.WorkspaceApproval) error {
	if s.Demo || c.Demo || a.Opportunity.Demo {
		return errors.New("demo contributions cannot execute")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var version int64
		if err := tx.QueryRowContext(ctx, "SELECT MAX(version) FROM config_versions").Scan(&version); err != nil {
			return err
		}
		if version != a.Config.Version {
			return ErrConflict
		}
		rows, err := tx.QueryContext(ctx, "SELECT data FROM contributions WHERE opportunity_id=?", c.OpportunityID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var b string
			var existing domain.Contribution
			if err = rows.Scan(&b); err != nil {
				rows.Close()
				return err
			}
			if err = json.Unmarshal([]byte(b), &existing); err != nil {
				rows.Close()
				return err
			}
			if existing.State != "FAILED" && existing.State != "ABANDONED" {
				rows.Close()
				return errors.New("this opportunity already has a contribution")
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO contributions VALUES (?,?,?)", c.ID, c.OpportunityID, string(b)); err != nil {
			return err
		}
		b, err = json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO workspace_approvals VALUES (?,?,?)", c.ID, a.Token, string(b)); err != nil {
			return err
		}
		return s.event(ctx, tx, "ContributionApproved", c.ID, "user", "Human approved isolated workspace preparation", a)
	})
}
func (s *Store) WorkspaceEvent(ctx context.Context, id, kind, message string, data any) error {
	return s.transact(ctx, func(tx *sql.Tx) error { return s.event(ctx, tx, kind, id, "workspace-manager", message, data) })
}
func (s *Store) WorkspaceMessage(ctx context.Context, id, message string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		var b string
		if err := tx.QueryRowContext(ctx, "SELECT data FROM contributions WHERE id=?", id).Scan(&b); err != nil {
			return err
		}
		var c domain.Contribution
		if err := json.Unmarshal([]byte(b), &c); err != nil {
			return err
		}
		c.Message = message
		c.UpdatedAt = time.Now().UTC()
		body, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE contributions SET data=? WHERE id=?", string(body), id); err != nil {
			return err
		}
		return s.event(ctx, tx, "WorkspaceStatus", id, "workspace-manager", message, nil)
	})
}
func (s *Store) RecoverWorkspaces(ctx context.Context) error {
	cs, err := s.Contributions(ctx)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if !c.Demo && (c.State == "SELECTED" || c.State == "PREPARING" || c.State == "ANALYZING_REPOSITORY") {
			if err = s.Transition(ctx, c.ID, "BLOCKED"); err != nil {
				return err
			}
			if err = s.WorkspaceMessage(ctx, c.ID, "Preparation interrupted by server restart. Workspace and audit history retained; inspect before retrying."); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) RelocateWorkspace(ctx context.Context, id, path string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		var b string
		if err := tx.QueryRowContext(ctx, "SELECT data FROM contributions WHERE id=?", id).Scan(&b); err != nil {
			return err
		}
		var c domain.Contribution
		if err := json.Unmarshal([]byte(b), &c); err != nil {
			return err
		}
		old := c.Workspace
		c.Workspace = path
		if !c.ExecutionApproved && c.State == "BLOCKED" && c.PreviousState == "PLANNING" {
			c.Message = "Repository cloned and branch verified. Workspace relocated; approve Start coding to run Codex, real verification and independent review."
		}
		c.UpdatedAt = time.Now().UTC()
		data, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE contributions SET data=? WHERE id=?", string(data), id); err != nil {
			return err
		}
		return s.event(ctx, tx, "WorkspaceRelocated", id, "user", "Contribution workspace relocated; original snapshots and history retained", map[string]string{"from": old, "to": path})
	})
}
