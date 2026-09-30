package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// monitorDatabase checks the SQLite schema through the same pool used by the
// application. If the open database becomes unusable, report causes the
// service to shut down instead of continuing to serve persistent 500 errors.
func monitorDatabase(ctx context.Context, db *sql.DB, interval time.Duration, report func(error)) {
	check := func() bool {
		var schemaVersion int
		err := db.QueryRowContext(ctx, `PRAGMA schema_version`).Scan(&schemaVersion)
		if err == nil {
			return true
		}
		if ctx.Err() == nil {
			report(fmt.Errorf("check SQLite availability: %w", err))
		}
		return false
	}

	if !check() {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !check() {
				return
			}
		}
	}
}
