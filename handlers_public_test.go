package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func useTestStore(t *testing.T) *Store {
	t.Helper()

	originalStore := store
	originalPublicBaseURL := publicBaseURL
	store = newBinStore()
	publicBaseURL = "https://hooklook.example"
	t.Cleanup(func() {
		store = originalStore
		publicBaseURL = originalPublicBaseURL
	})

	return store
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	health(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if body := rec.Body.String(); body != "OK, I'm healthy\n" {
		t.Errorf("body = %q, want %q", body, "OK, I'm healthy\n")
	}
}

func TestCreateBin(t *testing.T) {
	useTestStore(t)

	req := httptest.NewRequest(http.MethodPost, "/api/bins", nil)
	rec := httptest.NewRecorder()

	createBin(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var response struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code == "" {
		t.Fatal("response code is empty")
	}
	if want := publicBaseURL + "/b/" + response.Code; response.URL != want {
		t.Errorf("url = %q, want %q", response.URL, want)
	}
}

func TestCaptureRequest(t *testing.T) {
	store := useTestStore(t)
	bin := Bin{Code: "test-bin", Requests: make(map[string]ParsedRequest)}
	store.Bins[bin.Code] = bin

	start := time.Now().UTC()
	req := httptest.NewRequest("REPORT", "/b/test-bin/github/events?source=example", nil)
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)
	end := time.Now().UTC()

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var response struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Code) != requestCodeLength {
		t.Errorf("request code length = %d, want %d", len(response.Code), requestCodeLength)
	}
	if want := publicBaseURL + "/bins/test-bin/requests/" + response.Code; response.URL != want {
		t.Errorf("url = %q, want %q", response.URL, want)
	}

	captured, ok := store.Bins[bin.Code].Requests[response.Code]
	if !ok {
		t.Fatalf("request %q was not stored", response.Code)
	}
	if captured.Id != response.Code {
		t.Errorf("captured ID = %q, want %q", captured.Id, response.Code)
	}
	if captured.Method != "REPORT" {
		t.Errorf("captured method = %q, want %q", captured.Method, "REPORT")
	}
	if captured.Path != "/github/events" {
		t.Errorf("captured path = %q, want %q", captured.Path, "/github/events")
	}
	if captured.ReceiptTime.Before(start.Truncate(time.Second)) || captured.ReceiptTime.After(end) {
		t.Errorf("receipt time = %s, want it between %s and %s", captured.ReceiptTime, start, end)
	}
}

func TestCaptureRequestExactPath(t *testing.T) {
	store := useTestStore(t)
	bin := Bin{Code: "test-bin", Requests: make(map[string]ParsedRequest)}
	store.Bins[bin.Code] = bin

	req := httptest.NewRequest(http.MethodPost, "/b/test-bin", nil)
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	for _, captured := range store.Bins[bin.Code].Requests {
		if captured.Path != "" {
			t.Errorf("captured path = %q, want empty path", captured.Path)
		}
	}
}

func TestCaptureRequestRejectsUnknownBin(t *testing.T) {
	useTestStore(t)

	req := httptest.NewRequest(http.MethodPost, "/b/missing", nil)
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetBinRequests(t *testing.T) {
	store := useTestStore(t)
	bin := Bin{Code: "test-bin", Requests: map[string]ParsedRequest{
		"request1": {
			Id:          "request1",
			Method:      POST,
			Path:        "/github/events",
			ReceiptTime: time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC),
			RawQuery:    "secret=value",
			Headers:     HeaderMap{"Authorization": {redactedValue}},
			ContentType: "application/json",
			RawBody:     []byte("secret body"),
		},
	}}
	store.Bins[bin.Code] = bin

	req := httptest.NewRequest(http.MethodGet, "/api/bins/test-bin/requests", nil)
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var response []SummarizedRequest
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != 1 {
		t.Fatalf("request count = %d, want 1", len(response))
	}
	want := SummarizedRequest{
		Id:          "request1",
		Method:      POST,
		Path:        "/github/events",
		ReceiptTime: bin.Requests["request1"].ReceiptTime,
	}
	if response[0] != want {
		t.Errorf("request = %#v, want %#v", response[0], want)
	}
}

func TestGetBinRequestsRejectsUnknownBin(t *testing.T) {
	useTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/api/bins/missing/requests", nil)
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
