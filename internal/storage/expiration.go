package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) ExpireConfigs(ctx context.Context, at time.Time) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT version,restore_version,expires_at FROM config_expirations WHERE status='PENDING'")
		if err != nil {
			return err
		}
		type expiration struct {
			version, restore int64
			at               time.Time
		}
		due := []expiration{}
		for rows.Next() {
			var v expiration
			var stamp string
			if err = rows.Scan(&v.version, &v.restore, &stamp); err != nil {
				rows.Close()
				return err
			}
			v.at, err = time.Parse(time.RFC3339Nano, stamp)
			if err != nil {
				rows.Close()
				return err
			}
			if !v.at.After(at) {
				due = append(due, v)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, v := range due {
			var current int64
			if err = tx.QueryRowContext(ctx, "SELECT MAX(version) FROM config_versions").Scan(&current); err != nil {
				return err
			}
			status := "SUPERSEDED"
			if current == v.version {
				old, e := scanConfig(tx.QueryRowContext(ctx, "SELECT version,config,actor,reason,created_at FROM config_versions WHERE version=?", v.restore))
				if e != nil {
					return e
				}
				if _, e = insertConfig(ctx, tx, old.Config, "system", fmt.Sprintf("Temporary config v%d expired; restore v%d", v.version, v.restore)); e != nil {
					return e
				}
				status = "RESTORED"
			}
			if _, err = tx.ExecContext(ctx, "UPDATE config_expirations SET status=? WHERE version=?", status, v.version); err != nil {
				return err
			}
			if err = s.event(ctx, tx, "TemporaryConfigExpired", "config", "system", "Temporary configuration "+status, map[string]any{"version": v.version, "restore_version": v.restore}); err != nil {
				return err
			}
		}
		return nil
	})
}
