package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func useTestStore(t *testing.T) *Store {
	t.Helper()
	originalStore, originalURL, originalLimit, originalEventHub := store, publicBaseURL, maxRequestBodyBytes, eventHub
	store = newTestStore(t)
	eventHub = newEventHub()
	publicBaseURL, maxRequestBodyBytes = "https://hooklook.example", defaultMaxRequestBodyBytes
	t.Cleanup(func() {
		store, publicBaseURL, maxRequestBodyBytes, eventHub = originalStore, originalURL, originalLimit, originalEventHub
	})
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

func TestCreateBinRequiresValidCreationToken(t *testing.T) {
	store := useTestStore(t)

	for _, token := range []string{"", "not-a-creation-token"} {
		req := httptest.NewRequest(http.MethodPost, "/api/bins", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("token %q response = %d, %q", token, rec.Code, rec.Header().Get("WWW-Authenticate"))
		}
	}

	var binCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM bins`).Scan(&binCount); err != nil || binCount != 0 {
		t.Errorf("created bins = %d, error = %v; want 0, nil", binCount, err)
	}
}

func TestCreateBinConsumesCreationTokenAndRejectsExhaustedOrRevokedTokens(t *testing.T) {
	store := useTestStore(t)
	creationToken, token, err := store.issueCreationToken("test", 2)
	if err != nil {
		t.Fatalf("issue creation token: %v", err)
	}

	createBinWithToken := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/bins", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		routes().ServeHTTP(rec, req)
		return rec
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if rec := createBinWithToken(token); rec.Code != http.StatusCreated {
			t.Fatalf("creation attempt %d status = %d, want %d: %s", attempt, rec.Code, http.StatusCreated, rec.Body.String())
		}
	}
	if rec := createBinWithToken(token); rec.Code != http.StatusUnauthorized {
		t.Errorf("exhausted token status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var useCount int
	if err := store.db.QueryRow(`SELECT use_count FROM creation_tokens WHERE id = ?`, creationToken.ID).Scan(&useCount); err != nil || useCount != 2 {
		t.Errorf("token use count = %d, error = %v; want 2, nil", useCount, err)
	}

	revokedCreationToken, revokedToken, err := store.issueCreationToken("revoked", 1)
	if err != nil {
		t.Fatalf("issue revocable creation token: %v", err)
	}
	if err := store.revokeCreationToken(revokedCreationToken.ID); err != nil {
		t.Fatalf("revoke creation token: %v", err)
	}
	if rec := createBinWithToken(revokedToken); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked token status = %d, want %d", rec.Code, http.StatusUnauthorized)
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

func TestGetBinEventsStreamsPersistedRequestSummary(t *testing.T) {
	store := useTestStore(t)
	insertTestBin(t, store, "test-bin")
	server := httptest.NewServer(routes())
	t.Cleanup(server.Close)

	response, err := server.Client().Get(server.URL + "/api/bins/test-bin/events")
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

func TestDeletingBinClosesItsEventStream(t *testing.T) {
	store := useTestStore(t)
	insertTestBin(t, store, "test-bin")
	originalAdminToken := adminToken
	adminToken = testAdminToken
	t.Cleanup(func() { adminToken = originalAdminToken })

	server := httptest.NewServer(routes())
	t.Cleanup(server.Close)
	response, err := server.Client().Get(server.URL + "/api/bins/test-bin/events")
	if err != nil {
		t.Fatalf("open event stream: %v", err)
	}
	t.Cleanup(func() { response.Body.Close() })

	deleteRequest, err := http.NewRequest(http.MethodDelete, server.URL+"/admin/bins/test-bin", nil)
	if err != nil {
		t.Fatalf("create delete request: %v", err)
	}
	deleteRequest.Header.Set("Authorization", "Bearer "+testAdminToken)
	deleteResponse, err := server.Client().Do(deleteRequest)
	if err != nil {
		t.Fatalf("delete bin: %v", err)
	}
	deleteResponse.Body.Close()
	if deleteResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleteResponse.StatusCode)
	}

	if _, err := bufio.NewReader(response.Body).ReadString('\n'); err != io.EOF {
		t.Errorf("stream read after deletion error = %v, want EOF", err)
	}
}
