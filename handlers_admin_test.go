package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAllBins(t *testing.T) {
	store := useTestStore(t)
	insertTestBin(t, store, "first-bin")
	if _, err := store.saveRequest(ParsedRequest{RawBody: []byte{}}, "first-bin"); err != nil {
		t.Fatalf("save request: %v", err)
	}
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/bins", nil))
	var response []BinSummary
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil || rec.Code != http.StatusOK || len(response) != 1 || response[0].Code != "first-bin" || response[0].RequestCount != 1 {
		t.Errorf("response = %#v, status = %d, error = %v", response, rec.Code, err)
	}
}
