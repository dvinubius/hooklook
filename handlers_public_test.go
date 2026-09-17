package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func useTestStore(t *testing.T) *Store {
	t.Helper()
	originalStore, originalURL, originalEventHub := store, publicBaseURL, eventHub
	store = newTestStore(t)
	eventHub = newEventHub()
	publicBaseURL = "https://hooklook.example"
	t.Cleanup(func() {
		store, publicBaseURL, eventHub = originalStore, originalURL, originalEventHub
	})
	return store
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	health(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "OK, I'm healthy\n" {
		t.Errorf("health response = %d %q", rec.Code, rec.Body.String())
	}
}

func TestRemovedCreationAndTokenRoutes(t *testing.T) {
	useTestStore(t)
	for _, test := range []struct{ method, path string }{
		{http.MethodPost, "/api/bins"},
		{http.MethodPost, "/admin/tokens"},
	} {
		rec := httptest.NewRecorder()
		routes().ServeHTTP(rec, httptest.NewRequest(test.method, test.path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want 404", test.method, test.path, rec.Code)
		}
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
	authorizeTestBin(t, store, "test-bin")
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
	req := httptest.NewRequest(http.MethodGet, "/api/bins/test-bin/requests", nil)
	req.AddCookie(&http.Cookie{Name: ownerCookieName, Value: "test-owner"})
	routes().ServeHTTP(rec, req)
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

func TestGetBinEventsStreamsPersistedRequestSummary(t *testing.T) {
	store := useTestStore(t)
	insertTestBin(t, store, "test-bin")
	authorizeTestBin(t, store, "test-bin")
	server := httptest.NewServer(routes())
	t.Cleanup(server.Close)

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/bins/test-bin/events", nil)
	req.AddCookie(&http.Cookie{Name: ownerCookieName, Value: "test-owner"})
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("open event stream: %v", err)
	}
	t.Cleanup(func() { response.Body.Close() })
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("event stream response = %d, %q", response.StatusCode, response.Header.Get("Content-Type"))
	}

	captureResponse, err := server.Client().Post(server.URL+"/b/test-bin/github/events?source=example", "application/json", bytes.NewBufferString(`{"ok":true}`))
	if err != nil {
		t.Fatalf("capture request: %v", err)
	}
	captureResponse.Body.Close()
	if captureResponse.StatusCode != http.StatusCreated {
		t.Fatalf("capture status = %d", captureResponse.StatusCode)
	}

	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	if err != nil || line != "event: request\n" {
		t.Fatalf("event line = %q, error = %v", line, err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read event data: %v", err)
	}
	var summary SummarizedRequest
	if err := json.Unmarshal(bytes.TrimSuffix([]byte(line), []byte("\n"))[len("data: "):], &summary); err != nil {
		t.Fatalf("decode event data %q: %v", line, err)
	}
	if summary.Id != "1" || summary.Method != POST || summary.Path != "/github/events" || summary.RawQuery != "source=example" {
		t.Errorf("event summary = %#v", summary)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "\n" {
		t.Errorf("event terminator = %q, error = %v", line, err)
	}
}

func TestGetBinEventsRejectsUnknownBin(t *testing.T) {
	useTestStore(t)
	recorder := httptest.NewRecorder()
	routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/bins/missing/events", nil))
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func authorizeTestBin(t *testing.T, store *Store, code string) {
	t.Helper()
	if _, err := store.db.Exec(`UPDATE bins SET owner_digest = ? WHERE code = ?`, digestSecret("test-owner"), code); err != nil {
		t.Fatal(err)
	}
}
