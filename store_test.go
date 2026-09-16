package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := openDB(t.TempDir() + "/hooklook.db")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return newBinStore(db)
}

func insertTestBin(t *testing.T, store *Store, code string) Bin {
	t.Helper()
	bin := Bin{Code: code, CreatedAt: time.Now().UTC().Truncate(time.Second)}
	bin.ExpiresAt = bin.CreatedAt.Add(defaultBinTTL)
	if _, err := store.db.Exec(`INSERT INTO bins (code, created_at, expires_at, total_stored_body_kib) VALUES (?, ?, ?, ?)`, bin.Code, bin.CreatedAt.Format(time.RFC3339Nano), bin.ExpiresAt.Unix(), 0); err != nil {
		t.Fatalf("insert test bin: %v", err)
	}
	return bin
}

func TestGenerateBinCode(t *testing.T) {
	for range 100 {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("generate bin code: %v", err)
		}
		if len(code) != binCodeLength || strings.Trim(code, storeCodeAlphabet) != "" {
			t.Errorf("invalid bin code %q", code)
		}
	}
}

func TestCreateRetriesCodeCollision(t *testing.T) {
	store := newTestStore(t)
	existing, fresh := strings.Repeat("A", binCodeLength), strings.Repeat("B", binCodeLength)
	insertTestBin(t, store, existing)
	codes := []string{existing, fresh}
	store.generateCode = func() (string, error) { code := codes[0]; codes = codes[1:]; return code, nil }

	bin, err := store.createBin()
	if err != nil || bin.Code != fresh {
		t.Errorf("create bin = %#v, %v; want %q, nil", bin, err, fresh)
	}
}

func TestCreateReturnsCodeGenerationError(t *testing.T) {
	store := newTestStore(t)
	want := errors.New("randomness unavailable")
	store.generateCode = func() (string, error) { return "", want }
	if _, err := store.createBin(); !errors.Is(err, want) {
		t.Errorf("create error = %v, want wrapped %v", err, want)
	}
}

func TestCreateStoresBinWithExpiry(t *testing.T) {
	store := newTestStore(t)
	bin, err := store.createBin()
	if err != nil {
		t.Fatalf("create bin: %v", err)
	}
	var createdAt string
	var expiresAt int64
	if err := store.db.QueryRow(`SELECT created_at, expires_at FROM bins WHERE code = ?`, bin.Code).Scan(&createdAt, &expiresAt); err != nil {
		t.Fatalf("read stored bin: %v", err)
	}
	if createdAt != bin.CreatedAt.Format(time.RFC3339Nano) || expiresAt != bin.ExpiresAt.Unix() {
		t.Errorf("stored times = %q, %d", createdAt, expiresAt)
	}
}

func TestSaveRequestPersistsRequestAndAssignsSQLiteID(t *testing.T) {
	store := newTestStore(t)
	insertTestBin(t, store, "bin")
	receivedAt := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	id, err := store.saveRequest(ParsedRequest{Method: POST, Path: "/github/events", ReceiptTime: receivedAt, RawQuery: "source=example", Headers: HeaderMap{"X-Event": {"push"}}, ContentType: "application/json", RawBody: []byte(`{"ok":true}`), BodySizeKiB: 1}, "bin")
	if err != nil || id != "1" {
		t.Fatalf("save request = %q, %v; want 1, nil", id, err)
	}
	var method, path, query, contentType string
	var body []byte
	var bodyKiB int
	if err := store.db.QueryRow(`SELECT method, path, raw_query, content_type, raw_body, body_size_kib FROM requests WHERE id = 1`).Scan(&method, &path, &query, &contentType, &body, &bodyKiB); err != nil {
		t.Fatalf("read stored request: %v", err)
	}
	if method != "POST" || path != "/github/events" || query != "source=example" || contentType != "application/json" || string(body) != `{"ok":true}` || bodyKiB != 1 {
		t.Errorf("stored request = %q %q %q %q %q %d", method, path, query, contentType, body, bodyKiB)
	}
}

func TestSaveRequestRejectsUnknownOrFullBin(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.saveRequest(ParsedRequest{}, "missing"); !errors.Is(err, ErrBinNotFound) {
		t.Errorf("missing bin error = %v", err)
	}
	bin := insertTestBin(t, store, "full")
	if _, err := store.db.Exec(`UPDATE bins SET total_stored_body_kib = ? WHERE code = ?`, maxStoredBodyKiB, bin.Code); err != nil {
		t.Fatalf("fill bin budget: %v", err)
	}
	if _, err := store.saveRequest(ParsedRequest{BodySizeKiB: 1}, bin.Code); !errors.Is(err, ErrBinFull) {
		t.Errorf("full bin error = %v", err)
	}
}

func TestGetBinRequestsReturnsEmptySlice(t *testing.T) {
	store := newTestStore(t)
	insertTestBin(t, store, "bin")
	requests, err := store.getBinRequests("bin")
	if err != nil || requests == nil || len(requests) != 0 {
		t.Errorf("requests = %#v, error = %v", requests, err)
	}
}
