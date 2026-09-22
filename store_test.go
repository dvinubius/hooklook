package main

import (
	"errors"
	"regexp"
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
	s := newBinStore(db)
	if err := s.configureCapacity(defaultMaxStore); err != nil {
		t.Fatalf("configure test capacity: %v", err)
	}
	return s
}

func insertTestBin(t *testing.T, store *Store, code string) Bin {
	t.Helper()
	bin := Bin{Code: code, CreatedAt: time.Now().UTC().Truncate(time.Second)}
	bin.ExpiresAt = bin.CreatedAt.Add(defaultBinTTL)
	if _, err := store.db.Exec(`INSERT INTO bins (code, created_at, expires_at, total_body_bytes) VALUES (?, ?, ?, ?)`, bin.Code, bin.CreatedAt.Format(time.RFC3339Nano), bin.ExpiresAt.Unix(), 0); err != nil {
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
		if !regexp.MustCompile(`^[a-z]+-[a-z]+-[0-9]{2}$`).MatchString(code) {
			t.Errorf("invalid bin code %q", code)
		}
	}
}

func TestBinCodeWordsAreDistinct(t *testing.T) {
	for name, words := range map[string][]string{"adjectives": codeAdjectives[:], "nouns": codeNouns[:]} {
		seen := map[string]bool{}
		for _, word := range words {
			if seen[word] {
				t.Errorf("duplicate %s word %q", name, word)
			}
			seen[word] = true
		}
		if len(seen) != 36 {
			t.Errorf("%s has %d distinct words, want 36", name, len(seen))
		}
	}
}

func TestCreateRetriesCodeCollision(t *testing.T) {
	store := newTestStore(t)
	existing, fresh := "amber-otter-12", "silver-comet-87"
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
	var totalBodyBytes int64
	if err := store.db.QueryRow(`SELECT method, path, raw_query, content_type, raw_body, body_size_kib FROM requests WHERE id = 1`).Scan(&method, &path, &query, &contentType, &body, &bodyKiB); err != nil {
		t.Fatalf("read stored request: %v", err)
	}
	if err := store.db.QueryRow(`SELECT total_body_bytes FROM bins WHERE code = 'bin'`).Scan(&totalBodyBytes); err != nil {
		t.Fatalf("read bin byte count: %v", err)
	}
	if method != "POST" || path != "/github/events" || query != "source=example" || contentType != "application/json" || string(body) != `{"ok":true}` || bodyKiB != 1 {
		t.Errorf("stored request = %q %q %q %q %q %d", method, path, query, contentType, body, bodyKiB)
	}
	if totalBodyBytes != int64(len(body)) {
		t.Errorf("total body bytes = %d, want %d", totalBodyBytes, len(body))
	}
}

func TestSaveRequestRejectsUnknownOrFullBin(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.saveRequest(ParsedRequest{}, "missing"); !errors.Is(err, ErrBinNotFound) {
		t.Errorf("missing bin error = %v", err)
	}
	bin := insertTestBin(t, store, "full")
	if _, err := store.db.Exec(`UPDATE bins SET total_body_bytes = ? WHERE code = ?`, maxStoredBodyBytes, bin.Code); err != nil {
		t.Fatalf("fill bin budget: %v", err)
	}
	if _, err := store.saveRequest(ParsedRequest{RawBody: []byte("x")}, bin.Code); !errors.Is(err, ErrBinFull) {
		t.Errorf("full bin error = %v", err)
	}
}

func TestSaveRequestCountsOnlyExactBodyBytes(t *testing.T) {
	store := newTestStore(t)
	insertTestBin(t, store, "bin")
	if _, err := store.db.Exec(`UPDATE bins SET total_body_bytes = ? WHERE code = 'bin'`, maxStoredBodyBytes-2); err != nil {
		t.Fatal(err)
	}
	request := ParsedRequest{
		Headers: HeaderMap{"X-Large": {strings.Repeat("h", 40_000)}},
		RawBody: []byte("xy"),
	}
	if _, err := store.saveRequest(request, "bin"); err != nil {
		t.Fatalf("capture at exact body-byte limit: %v", err)
	}
	if _, err := store.saveRequest(ParsedRequest{RawBody: []byte("z")}, "bin"); !errors.Is(err, ErrBinFull) {
		t.Errorf("capture over body-byte limit error = %v", err)
	}
	var total int64
	if err := store.db.QueryRow(`SELECT total_body_bytes FROM bins WHERE code = 'bin'`).Scan(&total); err != nil || total != maxStoredBodyBytes {
		t.Errorf("total body bytes = %d, error = %v", total, err)
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
