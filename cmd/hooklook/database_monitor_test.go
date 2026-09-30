package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMonitorDatabaseReportsClosedDatabase(t *testing.T) {
	db, err := openDB(t.TempDir() + "/hooklook.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reported := make(chan error, 1)
	monitorDatabase(context.Background(), db, time.Hour, func(err error) {
		reported <- err
	})

	select {
	case err := <-reported:
		if !strings.Contains(err.Error(), "database is closed") {
			t.Fatalf("reported error = %q, want closed database error", err)
		}
	default:
		t.Fatal("closed database was not reported")
	}
}
