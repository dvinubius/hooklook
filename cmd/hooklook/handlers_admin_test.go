package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAllBinsRequiresOperatorBearerToken(t *testing.T) {
	store := useTestStore(t)
	original := adminToken
	adminToken = "operator-test-secret"
	t.Cleanup(func() { adminToken = original })
	insertTestBin(t, store, "bin")
	if _, err := store.saveRequest(ParsedRequest{RawBody: []byte("abc")}, "bin"); err != nil {
		t.Fatal(err)
	}

	for _, authorization := range []string{"", "Bearer wrong", "Basic operator-test-secret"} {
		req := httptest.NewRequest(http.MethodGet, "/admin/bins", nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("authorization %q: status=%d challenge=%q", authorization, rec.Code, rec.Header().Get("WWW-Authenticate"))
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/bins", nil)
	req.Header.Set("Authorization", "Bearer operator-test-secret")
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)
	var bins []BinSummary
	if err := json.NewDecoder(rec.Body).Decode(&bins); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || len(bins) != 1 ||
		bins[0].Code != "bin" || bins[0].RequestCount != 1 ||
		bins[0].TotalBodyBytes != 3 {
		t.Errorf("status=%d bins=%#v", rec.Code, bins)
	}
}

func TestGetStorageStatsRequiresOperatorBearerToken(t *testing.T) {
	useTestStore(t)
	original := adminToken
	adminToken = "operator-test-secret"
	t.Cleanup(func() { adminToken = original })

	unauthorized := httptest.NewRecorder()
	routes().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/admin/storage", nil))
	if unauthorized.Code != http.StatusUnauthorized || unauthorized.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("unauthorized response: status=%d challenge=%q", unauthorized.Code, unauthorized.Header().Get("WWW-Authenticate"))
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/storage", nil)
	req.Header.Set("Authorization", "Bearer operator-test-secret")
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)
	var stats GlobalStorageStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("response: status=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if stats.MaxBytes <= 0 || stats.DatabaseBytes <= 0 {
		t.Errorf("storage totals = %#v", stats)
	}
	if stats.UsedBytes != stats.DatabaseBytes-stats.ReusableBytes {
		t.Errorf("used bytes = %d, want %d", stats.UsedBytes, stats.DatabaseBytes-stats.ReusableBytes)
	}
	wantPercent := float64(stats.UsedBytes) / float64(stats.MaxBytes) * 100
	if stats.UsedPercent != wantPercent {
		t.Errorf("used percent = %f, want %f", stats.UsedPercent, wantPercent)
	}
}
