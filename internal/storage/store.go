package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"forgeflow/internal/config"
	"forgeflow/internal/contributions"
	"forgeflow/internal/domain"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sort"
	"time"
)

//go:embed migrations/*.sql
var migrations embed.FS
var ErrConflict = errors.New("configuration changed: refresh and propose again")
var ErrNotFound = errors.New("record not found")

type Store struct {
	db   *sql.DB
	Demo bool
}

func ID() string { return rand.Text() }
func Open(ctx context.Context, path string, demo bool) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, Demo: demo}
	if err = s.init(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) init(ctx context.Context) error {
	for _, statement := range []string{"PRAGMA foreign_keys=ON", "PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000", "CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL)"} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, file := range files {
		var count int
		if err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE name=?", file.Name()).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		body, err := migrations.ReadFile("migrations/" + file.Name())
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(body)); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations VALUES (?,?)", file.Name(), now())
		}
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var mode string
		err := tx.QueryRowContext(ctx, "SELECT value FROM metadata WHERE key='mode'").Scan(&mode)
		want := "live"
		if s.Demo {
			want = "demo"
		}
		if err == nil && mode != want {
			return fmt.Errorf("database mode is %s; refusing to open as %s", mode, want)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if errors.Is(err, sql.ErrNoRows) {
			if _, err = tx.ExecContext(ctx, "INSERT INTO metadata VALUES ('mode',?)", want); err != nil {
				return err
			}
			if _, err = insertConfig(ctx, tx, config.Default(), "system", "Initialize safe default profile"); err != nil {
				return err
			}
			return s.event(ctx, tx, "SystemInitialized", "system", "system", "Control plane initialized", map[string]string{"mode": want})
		}
		return nil
	})
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func (s *Store) transact(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) event(ctx context.Context, tx *sql.Tx, kind, entity, actor, message string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO events (type,entity_id,actor,message,data,created_at,demo) VALUES (?,?,?,?,?,?,?)", kind, entity, actor, message, string(b), now(), s.Demo)
	return err
}
func scanConfig(row interface{ Scan(...any) error }) (domain.ConfigVersion, error) {
	var v domain.ConfigVersion
	var body, created string
	err := row.Scan(&v.Version, &body, &v.Actor, &v.Reason, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	if err = json.Unmarshal([]byte(body), &v.Config); err != nil {
		return v, err
	}
	v.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	return v, err
}
func (s *Store) CurrentConfig(ctx context.Context) (domain.ConfigVersion, error) {
	return scanConfig(s.db.QueryRowContext(ctx, "SELECT version,config,actor,reason,created_at FROM config_versions ORDER BY version DESC LIMIT 1"))
}
func (s *Store) ConfigHistory(ctx context.Context) ([]domain.ConfigVersion, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT version,config,actor,reason,created_at FROM config_versions ORDER BY version DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ConfigVersion{}
	for rows.Next() {
		v, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func insertConfig(ctx context.Context, tx *sql.Tx, c domain.Config, actor, reason string) (int64, error) {
	body, err := json.Marshal(c)
	if err != nil {
		return 0, err
	}
	r, err := tx.ExecContext(ctx, "INSERT INTO config_versions (config,actor,reason,created_at) VALUES (?,?,?,?)", string(body), actor, reason, now())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}
func (s *Store) Propose(ctx context.Context, base int64, c domain.Config, reason string) (domain.Proposal, error) {
	return s.ProposeTimed(ctx, base, c, reason, nil)
}
func (s *Store) ProposeTimed(ctx context.Context, base int64, c domain.Config, reason string, expires *time.Time) (domain.Proposal, error) {
	p := domain.Proposal{ID: ID(), BaseVersion: base, Config: c, Reason: reason, Status: "PENDING", CreatedAt: time.Now().UTC()}
	p.ExpiresAt = expires
	if expires != nil && (expires.Before(time.Now()) || expires.After(time.Now().Add(365*24*time.Hour))) {
		return p, errors.New("expiration must be within the next year")
	}
	if err := config.Validate(c); err != nil {
		return p, err
	}
	if reason == "" || len(reason) > 1000 {
		return p, errors.New("a reason of at most 1000 characters is required")
	}
	body, err := json.Marshal(c)
	if err != nil {
		return p, err
	}
	err = s.transact(ctx, func(tx *sql.Tx) error {
		var current int64
		if err := tx.QueryRowContext(ctx, "SELECT MAX(version) FROM config_versions").Scan(&current); err != nil {
			return err
		}
		if current != base {
			return ErrConflict
		}
		expiry := ""
		if expires != nil {
			expiry = expires.UTC().Format(time.RFC3339Nano)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO config_proposals (id,base_version,config,reason,status,created_at,expires_at) VALUES (?,?,?,?,?,?,?)", p.ID, base, string(body), reason, p.Status, p.CreatedAt.Format(time.RFC3339Nano), expiry); err != nil {
			return err
		}
		return s.event(ctx, tx, "ConfigChangeProposed", p.ID, "user", reason, map[string]int64{"base_version": base})
	})
	return p, err
}
func (s *Store) Proposals(ctx context.Context) ([]domain.Proposal, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,base_version,config,reason,status,created_at,expires_at FROM config_proposals ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Proposal{}
	for rows.Next() {
		var p domain.Proposal
		var body, created, expires string
		if err = rows.Scan(&p.ID, &p.BaseVersion, &body, &p.Reason, &p.Status, &created, &expires); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(body), &p.Config); err != nil {
			return nil, err
		}
		if p.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, err
		}
		if expires != "" {
			v, e := time.Parse(time.RFC3339Nano, expires)
			if e != nil {
				return nil, e
			}
			p.ExpiresAt = &v
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
func (s *Store) Apply(ctx context.Context, id string) (domain.ConfigVersion, error) {
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var body, reason, status, expires string
		var base, current int64
		err := tx.QueryRowContext(ctx, "SELECT config,reason,status,base_version,expires_at FROM config_proposals WHERE id=?", id).Scan(&body, &reason, &status, &base, &expires)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "PENDING" {
			return errors.New("proposal is not pending")
		}
		if expires != "" {
			deadline, e := time.Parse(time.RFC3339Nano, expires)
			if e != nil {
				return e
			}
			if !deadline.After(time.Now()) {
				return errors.New("proposal has already expired")
			}
		}
		if err = tx.QueryRowContext(ctx, "SELECT MAX(version) FROM config_versions").Scan(&current); err != nil {
			return err
		}
		if current != base {
			return ErrConflict
		}
		var c domain.Config
		if err = json.Unmarshal([]byte(body), &c); err != nil {
			return err
		}
		if err = config.Validate(c); err != nil {
			return err
		}
		version, err := insertConfig(ctx, tx, c, "user", reason)
		if err != nil {
			return err
		}
		if expires != "" {
			if _, err = tx.ExecContext(ctx, "INSERT INTO config_expirations VALUES (?,?,?,'PENDING')", version, base, expires); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE config_proposals SET status='APPLIED' WHERE id=?", id); err != nil {
			return err
		}
		return s.event(ctx, tx, "ConfigChanged", id, "user", reason, map[string]int64{"from": base, "to": version})
	})
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	return s.CurrentConfig(ctx)
}
func (s *Store) CancelProposal(ctx context.Context, id string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, "UPDATE config_proposals SET status='CANCELLED' WHERE id=? AND status='PENDING'", id)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrNotFound
		}
		return s.event(ctx, tx, "ConfigProposalCancelled", id, "user", "Configuration proposal cancelled", nil)
	})
}
func (s *Store) Rollback(ctx context.Context, version, base int64) (domain.ConfigVersion, error) {
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var current int64
		if err := tx.QueryRowContext(ctx, "SELECT MAX(version) FROM config_versions").Scan(&current); err != nil {
			return err
		}
		if current != base {
			return ErrConflict
		}
		v, err := scanConfig(tx.QueryRowContext(ctx, "SELECT version,config,actor,reason,created_at FROM config_versions WHERE version=?", version))
		if err != nil {
			return err
		}
		if err = config.Validate(v.Config); err != nil {
			return err
		}
		target, err := insertConfig(ctx, tx, v.Config, "user", fmt.Sprintf("Restore config v%d", version))
		if err != nil {
			return err
		}
		return s.event(ctx, tx, "ConfigChanged", "config", "user", fmt.Sprintf("Restored config v%d as v%d", version, target), map[string]int64{"restored_version": version, "from": base, "to": target})
	})
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	return s.CurrentConfig(ctx)
}
func listJSON[T any](ctx context.Context, db *sql.DB, query string) ([]T, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []T{}
	for rows.Next() {
		var body string
		if err = rows.Scan(&body); err != nil {
			return nil, err
		}
		var item T
		if err = json.Unmarshal([]byte(body), &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *Store) Opportunities(ctx context.Context) ([]domain.Opportunity, error) {
	return listJSON[domain.Opportunity](ctx, s.db, "SELECT data FROM opportunities ORDER BY score DESC,id")
}
func (s *Store) Contributions(ctx context.Context) ([]domain.Contribution, error) {
	return listJSON[domain.Contribution](ctx, s.db, "SELECT data FROM contributions ORDER BY id")
}
func (s *Store) Opportunity(ctx context.Context, id string) (domain.Opportunity, error) {
	var o domain.Opportunity
	var body string
	err := s.db.QueryRowContext(ctx, "SELECT data FROM opportunities WHERE id=?", id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return o, ErrNotFound
	}
	if err != nil {
		return o, err
	}
	err = json.Unmarshal([]byte(body), &o)
	return o, err
}
func (s *Store) Events(ctx context.Context, after int64, entity string, limit int) ([]domain.Event, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,type,entity_id,actor,message,data,created_at,demo FROM events WHERE id>? AND (?='' OR entity_id=?) ORDER BY id LIMIT ?", after, entity, entity, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Event{}
	for rows.Next() {
		var e domain.Event
		var data, created string
		if err = rows.Scan(&e.ID, &e.Type, &e.EntityID, &e.Actor, &e.Message, &data, &created, &e.Demo); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(data), &e.Data); err != nil {
			return nil, err
		}
		if e.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
func (s *Store) RecentEvents(ctx context.Context) ([]domain.Event, error) {
	var cursor int64
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM events").Scan(&cursor); err != nil {
		return nil, err
	}
	cursor -= 100
	if cursor < 0 {
		cursor = 0
	}
	return s.Events(ctx, cursor, "", 100)
}
func (s *Store) Transition(ctx context.Context, id, to string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		var body string
		err := tx.QueryRowContext(ctx, "SELECT data FROM contributions WHERE id=?", id).Scan(&body)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var c domain.Contribution
		if err = json.Unmarshal([]byte(body), &c); err != nil {
			return err
		}
		if err = contributions.Validate(c.State, to, c.PreviousState); err != nil {
			return err
		}
		from := c.State
		if to == "PAUSED" || to == "BLOCKED" {
			c.PreviousState = from
		} else {
			c.PreviousState = ""
		}
		c.State = to
		c.UpdatedAt = time.Now().UTC()
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE contributions SET data=? WHERE id=?", string(b), id); err != nil {
			return err
		}
		return s.event(ctx, tx, "ContributionStateChanged", id, "system", from+" → "+to, map[string]string{"from": from, "to": to})
	})
}

// SeedOnce is atomic and idempotent. It can never write to a live database.
func (s *Store) SeedOnce(ctx context.Context, opportunities []domain.Opportunity, contribs []domain.Contribution) error {
	if !s.Demo {
		return errors.New("seed requires explicit demo mode")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM metadata WHERE key='seeded'").Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		sort.Slice(opportunities, func(i, j int) bool { return opportunities[i].Ranking.Score > opportunities[j].Ranking.Score })
		for _, o := range opportunities {
			if !o.Demo {
				return errors.New("seed opportunity must be labeled demo")
			}
			b, err := json.Marshal(o)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO opportunities VALUES (?,?,?)", o.ID, string(b), o.Ranking.Score); err != nil {
				return err
			}
			if err = s.event(ctx, tx, "IssueRanked", o.ID, "demo-seed", "Illustrative opportunity: "+o.Repository+" — "+o.Title, map[string]any{"score": o.Ranking.Score}); err != nil {
				return err
			}
		}
		for _, c := range contribs {
			if !c.Demo || !contributions.Known(c.State) {
				return errors.New("invalid seed contribution")
			}
			b, err := json.Marshal(c)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO contributions VALUES (?,?,?)", c.ID, c.OpportunityID, string(b)); err != nil {
				return err
			}
			if err = s.event(ctx, tx, "DemoContributionLoaded", c.ID, "demo-seed", "Illustrative state: "+c.Repository+" — "+c.State, nil); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO metadata VALUES ('seeded','1')")
		return err
	})
}
