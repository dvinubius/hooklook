package main

import "testing"

func TestMigrateCreatesCurrentSchema(t *testing.T) {
	db, err := openDB(t.TempDir() + "/hooklook.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for range 2 {
		if err := migrate(db); err != nil {
			t.Fatalf("initialize schema: %v", err)
		}
	}
	if _, err := db.Exec(`
		INSERT INTO bins (code, created_at, expires_at, total_body_bytes)
		VALUES ('bin', '2026-09-17T00:00:00Z', 2000000000, 0)
	`); err != nil {
		t.Fatalf("insert bin: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bins`).Scan(&count); err != nil || count != 1 {
		t.Errorf("bin count = %d, error = %v", count, err)
	}
	var legacyCount int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'creation_tokens'
	`).Scan(&legacyCount); err != nil || legacyCount != 0 {
		t.Errorf("legacy token table count = %d, error = %v", legacyCount, err)
	}
}
