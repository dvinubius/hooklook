package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	db.SetMaxOpenConns(1)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// Schema creation is idempotent. The request index is also added to existing
// databases so aggregate observability queries stay bounded.
func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS bins (
			code TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			total_body_bytes INTEGER NOT NULL DEFAULT 0,
			owner_digest TEXT NOT NULL DEFAULT '',
			invite_id TEXT NOT NULL DEFAULT '',
			sharing_enabled INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bin_code TEXT NOT NULL,
			created_at TEXT NOT NULL,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			raw_query TEXT NOT NULL,
			headers_json BLOB NOT NULL,
			content_type TEXT NOT NULL,
			raw_body BLOB NOT NULL,
			body_size_kib INTEGER NOT NULL,
			FOREIGN KEY(bin_code) REFERENCES bins(code) ON DELETE CASCADE
		);
        CREATE INDEX IF NOT EXISTS requests_bin_code_idx ON requests(bin_code);
	`)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}
