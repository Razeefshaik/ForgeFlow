package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"regexp"
)

var discoveryScope = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s *Store) DiscoveryCursors(ctx context.Context, scope string) (map[string]domain.DiscoveryCursor, error) {
	if !discoveryScope.MatchString(scope) {
		return nil, errors.New("invalid discovery search scope")
	}
	var raw string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM metadata WHERE key=?", "discovery_cursor:"+scope).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]domain.DiscoveryCursor{}, nil
	}
	if err != nil {
		return nil, err
	}
	var v map[string]domain.DiscoveryCursor
	if err = json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, err
	}
	if v == nil {
		v = map[string]domain.DiscoveryCursor{}
	}
	if err = validateDiscoveryCursors(v); err != nil {
		return nil, err
	}
	return v, nil
}

func validateDiscoveryCursors(v map[string]domain.DiscoveryCursor) error {
	if len(v) > 50 {
		return errors.New("too many discovery cursors")
	}
	for lang, c := range v {
		if lang == "" || c.Page < 1 || c.Page > 40 || c.Offset < 0 || c.Offset >= 25 {
			return errors.New("invalid discovery search cursor")
		}
	}
	return nil
}

func saveDiscoveryCursors(ctx context.Context, tx *sql.Tx, scope string, v map[string]domain.DiscoveryCursor) error {
	if !discoveryScope.MatchString(scope) {
		return errors.New("invalid discovery search scope")
	}
	if err := validateDiscoveryCursors(v); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO metadata VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", "discovery_cursor:"+scope, string(b))
	return err
}
