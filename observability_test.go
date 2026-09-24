package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	for _, want := range []string{`hooklook_capture_results_total{result="accepted"} 1`, `hooklook_capture_results_total{result="missing_bin"} 1`, `hooklook_http_requests_total{method="OTHER",route="capture",status="201"} 1`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing metric %q", want)
		}
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
