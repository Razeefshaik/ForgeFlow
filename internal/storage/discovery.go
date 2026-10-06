package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"time"
)

func (s *Store) StartDiscovery(ctx context.Context, run domain.DiscoveryRun) error {
	if s.Demo {
		return errors.New("live discovery is unavailable in demo mode")
	}
	if run.Status != "RUNNING" || run.ConfigVersion < 1 || run.ID == "" {
		return errors.New("invalid discovery run")
	}
	b, err := json.Marshal(run)
	if err != nil {
		return err
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO discovery_runs VALUES (?,?,?)", run.ID, string(b), run.StartedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		return s.event(ctx, tx, "DiscoveryStarted", run.ID, "discovery", "GitHub discovery started", map[string]int64{"config_version": run.ConfigVersion})
	})
}

// FinishDiscovery commits new observations and completion events together. Previous events and unobserved issues survive.
func (s *Store) FinishDiscovery(ctx context.Context, run domain.DiscoveryRun, opportunities []domain.Opportunity) error {
	if s.Demo {
		return errors.New("live discovery is unavailable in demo mode")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var previous string
		if err := tx.QueryRowContext(ctx, "SELECT data FROM discovery_runs WHERE id=?", run.ID).Scan(&previous); err != nil {
			return err
		}
		var old domain.DiscoveryRun
		if err := json.Unmarshal([]byte(previous), &old); err != nil {
			return err
		}
		if old.Status != "RUNNING" {
			return errors.New("discovery run is already finished")
		}
		if run.ID != old.ID || run.ConfigVersion != old.ConfigVersion || !run.StartedAt.Equal(old.StartedAt) || run.FinishedAt == nil || run.Status == "RUNNING" {
			return errors.New("discovery run identity and snapshot are immutable")
		}
		for _, o := range opportunities {
			if o.Demo || o.Evidence == nil || o.Evidence.RunID != run.ID || o.Evidence.ConfigVersion != run.ConfigVersion {
				return errors.New("invalid discovery observation")
			}
			b, err := json.Marshal(o)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO opportunities VALUES (?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data,score=excluded.score", o.ID, string(b), o.Ranking.Score); err != nil {
				return err
			}
			if err = s.event(ctx, tx, "IssueRanked", o.ID, "discovery", "Analyzed "+o.Repository+" — "+o.Title, o); err != nil {
				return err
			}
		}
		b, err := json.Marshal(run)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE discovery_runs SET data=? WHERE id=?", string(b), run.ID); err != nil {
			return err
		}
		return s.event(ctx, tx, "DiscoveryFinished", run.ID, "discovery", "GitHub scan "+run.Status, run)
	})
}

func (s *Store) DiscoveryAutomatic(ctx context.Context) (*bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM metadata WHERE key='discovery_automatic'").Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	enabled := value == "true"
	return &enabled, nil
}
func (s *Store) SetDiscoveryAutomatic(ctx context.Context, enabled bool) error {
	if s.Demo {
		return errors.New("live discovery is disabled in demo mode")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		value := "false"
		if enabled {
			value = "true"
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO metadata VALUES ('discovery_automatic',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", value); err != nil {
			return err
		}
		return s.event(ctx, tx, "DiscoveryScheduleChanged", "discovery", "user", "Automatic discovery "+value, map[string]bool{"enabled": enabled})
	})
}
func (s *Store) RequestDiscoveryCancellation(ctx context.Context) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		return s.event(ctx, tx, "DiscoveryCancellationRequested", "discovery", "user", "Human requested scan cancellation", nil)
	})
}
func (s *Store) DiscoveryRuns(ctx context.Context) ([]domain.DiscoveryRun, error) {
	return s.discoveryRuns(ctx, "SELECT data FROM discovery_runs ORDER BY started_at DESC LIMIT 50")
}
func (s *Store) discoveryRuns(ctx context.Context, query string) ([]domain.DiscoveryRun, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []domain.DiscoveryRun{}
	for rows.Next() {
		var b string
		var run domain.DiscoveryRun
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(b), &run); err != nil {
			return nil, err
		}
		list = append(list, run)
	}
	return list, rows.Err()
}
func (s *Store) RecoverDiscovery(ctx context.Context) error {
	runs, err := s.discoveryRuns(ctx, "SELECT data FROM discovery_runs WHERE json_extract(data,'$.status')='RUNNING'")
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.Status == "RUNNING" {
			t := time.Now().UTC()
			run.FinishedAt = &t
			run.Status = "INTERRUPTED"
			run.Warnings = append(run.Warnings, "Server stopped before this scan completed; no successful completion is inferred.")
			if err = s.FinishDiscovery(ctx, run, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
