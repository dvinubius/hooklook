package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func useTestStore(t *testing.T) *Store {
	t.Helper()
	originalStore, originalURL, originalLimit := store, publicBaseURL, maxRequestBodyBytes
	store = newTestStore(t)
	publicBaseURL, maxRequestBodyBytes = "https://hooklook.example", defaultMaxRequestBodyBytes
	t.Cleanup(func() { store, publicBaseURL, maxRequestBodyBytes = originalStore, originalURL, originalLimit })
	return store
}

func TestCaptureRequestRejectsOversizedBodyWithoutSavingIt(t *testing.T) {
	store := useTestStore(t)
	insertTestBin(t, store, "test-bin")
	maxRequestBodyBytes = 4
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/b/test-bin", bytes.NewReader([]byte("12345"))))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&count); err != nil || count != 0 {
		t.Errorf("saved count = %d, error = %v", count, err)
	}
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	health(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "OK, I'm healthy\n" {
		t.Errorf("health response = %d %q", rec.Code, rec.Body.String())
	}
}

func TestCreateBin(t *testing.T) {
	useTestStore(t)
	rec := httptest.NewRecorder()
	createBin(rec, httptest.NewRequest(http.MethodPost, "/api/bins", nil))
	var response struct{ Code, URL string }
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil || rec.Code != http.StatusCreated || response.Code == "" || response.URL != publicBaseURL+"/b/"+response.Code {
		t.Errorf("response = %#v, status = %d, error = %v", response, rec.Code, err)
	}
}

func TestCaptureRequest(t *testing.T) {
	store := useTestStore(t)
	insertTestBin(t, store, "test-bin")
	start := time.Now().UTC()
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, httptest.NewRequest("REPORT", "/b/test-bin/github/events?source=example", nil))
	end := time.Now().UTC()
	var response struct{ ID, URL string }
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil || rec.Code != http.StatusCreated || response.ID != "1" || response.URL != publicBaseURL+"/bins/test-bin/requests/1" {
		t.Fatalf("response = %#v, status = %d, error = %v", response, rec.Code, err)
	}
	requests, err := store.getBinRequests("test-bin")
	if err != nil || len(requests) != 1 || requests[0].Method != "REPORT" || requests[0].Path != "/github/events" || requests[0].ReceivedAt.Before(start.Truncate(time.Second)) || requests[0].ReceivedAt.After(end) {
		t.Errorf("stored requests = %#v, error = %v", requests, err)
	}
}

func TestCaptureRequestExactPathAndUnknownBin(t *testing.T) {
	store := useTestStore(t)
	insertTestBin(t, store, "test-bin")
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/b/test-bin", nil))
	requests, err := store.getBinRequests("test-bin")
	if rec.Code != http.StatusCreated || err != nil || len(requests) != 1 || requests[0].Path != "" {
		t.Errorf("exact request = %#v, error = %v, status = %d", requests, err, rec.Code)
	}
	rec = httptest.NewRecorder()
	routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/b/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown bin status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetBinRequests(t *testing.T) {
	store := useTestStore(t)
	insertTestBin(t, store, "test-bin")
	receivedAt := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	if _, err := store.saveRequest(ParsedRequest{
		Method:      POST,
		Path:        "/github/events",
		ReceiptTime: receivedAt,
		RawQuery:    "delivery=123",
		Headers:     HeaderMap{"X-Event": {"push"}, "X-Trace-ID": {"abc", "def"}},
		ContentType: "application/json",
		RawBody:     []byte{},
		BodySizeKiB: 2,
	}, "test-bin"); err != nil {
		t.Fatalf("save request: %v", err)
	}
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/bins/test-bin/requests", nil))
	body := rec.Body.Bytes()
	var response []SummarizedRequest
	if err := json.Unmarshal(body, &response); err != nil || rec.Code != http.StatusOK || len(response) != 1 {
		t.Errorf("response = %#v, status = %d, error = %v", response, rec.Code, err)
		return
	}
	got := response[0]
	if got.Id != "1" || got.Method != POST || got.Path != "/github/events" ||
		got.RawQuery != "delivery=123" || !got.ReceivedAt.Equal(receivedAt) ||
		got.ContentType != "application/json" || got.BodySizeKiB != 2 || got.HeaderCount != 2 {
		t.Errorf("summary = %#v", got)
	}

	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	if _, exists := raw[0]["receiptTime"]; exists {
		t.Error("response contains deprecated receiptTime field")
	}
	for _, field := range []string{"rawQuery", "receivedAt", "contentType", "bodySizeKiB", "headerCount"} {
		if _, exists := raw[0][field]; !exists {
			t.Errorf("response is missing %q: %#v", field, raw[0])
		}
	}
}
