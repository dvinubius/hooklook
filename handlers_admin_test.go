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
