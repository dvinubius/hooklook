package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAllBins(t *testing.T) {
	store := useTestStore(t)
	store.Bins["first-bin"] = Bin{Code: "first-bin", Requests: make(map[string]ParsedRequest)}

	req := httptest.NewRequest(http.MethodGet, "/admin/bins", nil)
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var response []Bin
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != 1 {
		t.Fatalf("bin count = %d, want 1", len(response))
	}
	if response[0].Code != "first-bin" {
		t.Errorf("bin code = %q, want %q", response[0].Code, "first-bin")
	}
}
