package storage

import (
	"context"
	"database/sql"
	"modernc.org/sqlite"
	"time"
)

// SQLite settings are per connection. Reapply them if database/sql replaces
// a connection after an error; initialization alone protects only the first one.
func init() {
	driver := &sqlite.Driver{}
	driver.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, statement := range []string{"PRAGMA busy_timeout=5000", "PRAGMA foreign_keys=ON"} {
			if _, err := conn.ExecContext(ctx, statement, nil); err != nil {
				return err
			}
		}
		return nil
	})
	sql.Register("forgeflow-sqlite", driver)
}
