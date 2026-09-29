package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func TestPrivateMetricsUseBoundedLabelsAndCaptureOutcomes(t *testing.T) {
	s := useTestStore(t)
	original := telemetry
	telemetry = newTelemetry()
	t.Cleanup(func() { telemetry = original })
	insertTestBin(t, s, "secret-bin")
	for _, path := range []string{"/b/secret-bin/secret-path?token=secret-query", "/b/missing/secret-path?token=secret-query"} {
		rec := httptest.NewRecorder()
		routes().ServeHTTP(rec, httptest.NewRequest("REPORT", path, nil))
	}
	public := httptest.NewRecorder()
	routes().ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if public.Code != http.StatusNotFound {
		t.Fatalf("public metrics status = %d", public.Code)
	}
	private := httptest.NewRecorder()
	metricsHandler().ServeHTTP(private, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if private.Code != http.StatusOK {
		t.Fatalf("private metrics status = %d: %s", private.Code, private.Body.String())
	}
	body := private.Body.String()
	for _, want := range []string{`hooklook_capture_results_total{result="accepted"} 1`, `hooklook_capture_results_total{result="missing_bin"} 1`, `hooklook_http_requests_total{method="OTHER",route="capture",status="201"} 1`, `hooklook_db_operations_total{operation="capture",result="not_found"} 1`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing metric %q", want)
		}
	}
	if strings.Contains(body, "hooklook_db_errors_total{") {
		t.Error("missing-bin capture counted as a database error")
	}
	for _, secret := range []string{"secret-bin", "secret-path", "secret-query"} {
		if strings.Contains(body, secret) {
			t.Errorf("metrics exposed %q", secret)
		}
	}
}

func TestReadinessUsesSQLite(t *testing.T) {
	s := useTestStore(t)
	ready := httptest.NewRecorder()
	routes().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("ready status = %d", ready.Code)
	}
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	unavailable := httptest.NewRecorder()
	routes().ServeHTTP(unavailable, httptest.NewRequest(http.MethodGet, "/ready", nil).WithContext(context.Background()))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d", unavailable.Code)
	}
}

func TestCollectionReportsActiveBinsAndAvailability(t *testing.T) {
	s := useTestStore(t)
	original := telemetry
	telemetry = newTelemetry()
	t.Cleanup(func() { telemetry = original })
	insertTestBin(t, s, "observed-bin")
	collectTelemetry(context.Background(), s)
	rec := httptest.NewRecorder()
	metricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, want := range []string{"hooklook_active_bins 1", "hooklook_active_bins_collection_available 1", "hooklook_storage_collection_available 1"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	collectTelemetry(context.Background(), s)
	rec = httptest.NewRecorder()
	metricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, want := range []string{"hooklook_active_bins_collection_available 0", "hooklook_storage_collection_available 0"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("missing failure signal %q", want)
		}
	}
}

func TestDBMetricsSeparateDomainOutcomesFromFailures(t *testing.T) {
	original := telemetry
	telemetry = newTelemetry()
	t.Cleanup(func() { telemetry = original })
	sqliteFull := classifyStoreError(fmt.Errorf("insert request: %w", sqlite3.Error{Code: sqlite3.ErrFull}))
	if !errors.Is(sqliteFull, ErrStoreFull) {
		t.Fatalf("SQLite FULL not classified as ErrStoreFull: %v", sqliteFull)
	}
	for _, c := range []struct {
		operation string
		err       error
	}{
		{"capture", nil},
		{"capture", ErrBinNotFound},
		{"capture", ErrBinFull},
		{"capture", ErrStoreFull},
		{"capture", sqliteFull},
		{"create", ErrStoreFull},
		{"detail", ErrRequestNotFound},
		{"list", errors.New("disk I/O error")},
	} {
		observeDBOperation(c.operation, time.Now(), c.err)
	}
	rec := httptest.NewRecorder()
	metricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`hooklook_db_operations_total{operation="capture",result="success"} 1`,
		`hooklook_db_operations_total{operation="capture",result="not_found"} 1`,
		`hooklook_db_operations_total{operation="capture",result="capacity"} 2`,
		`hooklook_db_operations_total{operation="capture",result="error"} 1`,
		`hooklook_db_operations_total{operation="create",result="capacity"} 1`,
		`hooklook_db_operations_total{operation="detail",result="not_found"} 1`,
		`hooklook_db_operations_total{operation="list",result="error"} 1`,
		`hooklook_db_errors_total{kind="full",operation="capture"} 1`,
		`hooklook_db_errors_total{kind="other",operation="list"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing metric %q", want)
		}
	}
	if n := strings.Count(body, "\nhooklook_db_errors_total{"); n != 2 {
		t.Errorf("db error series = %d, want only the two real failures", n)
	}
}
