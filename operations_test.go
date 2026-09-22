package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func expireBin(t *testing.T, s *Store, code string, when time.Time) {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE bins SET expires_at = ? WHERE code = ?`, when.Unix(), code); err != nil {
		t.Fatal(err)
	}
}

func storedExpiry(t *testing.T, s *Store, code string) time.Time {
	t.Helper()
	var value int64
	if err := s.db.QueryRow(`SELECT expires_at FROM bins WHERE code = ?`, code).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return time.Unix(value, 0)
}

func TestUseRenewsOwnerGuestAndCaptureButNotRejectedAccess(t *testing.T) {
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	soon := time.Now().Add(time.Hour)
	expireBin(t, s, bin.Code, soon)
	if _, err := s.ownedBin(owner); err != nil {
		t.Fatal(err)
	}
	if got := storedExpiry(t, s, bin.Code); got.Before(time.Now().Add(defaultBinTTL - time.Minute)) {
		t.Errorf("owner expiry = %v", got)
	}
	expireBin(t, s, bin.Code, soon)
	if _, err := s.access(bin.Code, "", "wrong"); !errors.Is(err, ErrBinNotFound) {
		t.Errorf("invalid access = %v", err)
	}
	if got := storedExpiry(t, s, bin.Code); got.Unix() != soon.Unix() {
		t.Errorf("rejected access renewed to %v", got)
	}
	ownerAccess, err := s.access(bin.Code, owner, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.setSharing(bin.Code, true); err != nil {
		t.Fatal(err)
	}
	expireBin(t, s, bin.Code, soon)
	if _, err := s.access(bin.Code, "", ownerAccess.InviteID); err != nil {
		t.Fatal(err)
	}
	if got := storedExpiry(t, s, bin.Code); got.Before(time.Now().Add(defaultBinTTL - time.Minute)) {
		t.Errorf("guest expiry = %v", got)
	}
	expireBin(t, s, bin.Code, soon)
	if _, err := s.saveRequest(ParsedRequest{RawBody: []byte("ok")}, bin.Code); err != nil {
		t.Fatal(err)
	}
	if got := storedExpiry(t, s, bin.Code); got.Before(time.Now().Add(defaultBinTTL - time.Minute)) {
		t.Errorf("capture expiry = %v", got)
	}
	expireBin(t, s, bin.Code, time.Now().Add(-time.Second))
	if _, err := s.saveRequest(ParsedRequest{}, bin.Code); !errors.Is(err, ErrBinNotFound) {
		t.Errorf("expired capture = %v", err)
	}
	if got := callInspector(t, "GET", "/b/"+bin.Code, "", "").Code; got != http.StatusNotFound {
		t.Errorf("expired HTTP capture = %d", got)
	}
	expiredAPI := callInspector(t, "GET", "/api/bins/"+bin.Code, "", owner)
	if expiredAPI.Code != http.StatusNotFound || expiredAPI.Header().Get("X-Hooklook-Error") != "bin_expired" {
		t.Errorf("expired API = %d, error = %q", expiredAPI.Code, expiredAPI.Header().Get("X-Hooklook-Error"))
	}
	expiredGuestAPI := callInspector(t, "GET", "/api/bins/"+bin.Code+"?invite="+ownerAccess.InviteID, "", "")
	if expiredGuestAPI.Code != http.StatusNotFound || expiredGuestAPI.Header().Get("X-Hooklook-Error") != "shared_bin_unavailable" {
		t.Errorf("expired guest API = %d, error = %q", expiredGuestAPI.Code, expiredGuestAPI.Header().Get("X-Hooklook-Error"))
	}
	if got := callInspector(t, "GET", "/bins/"+bin.Code, "", owner).Code; got != http.StatusNotFound {
		t.Errorf("expired page = %d", got)
	}
}

func TestCleanupCascadesAndClosesStreams(t *testing.T) {
	s := useTestStore(t)
	bin, _, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.saveRequest(ParsedRequest{RawBody: []byte("body")}, bin.Code); err != nil {
		t.Fatal(err)
	}
	stream, ok := eventHub.subscribe(bin.Code, true)
	if !ok {
		t.Fatal("subscribe")
	}
	expireBin(t, s, bin.Code, time.Now().Add(-time.Second))
	codes, err := s.deleteExpired(time.Now())
	if err != nil || len(codes) != 1 || codes[0] != bin.Code {
		t.Fatalf("deleted codes = %v, %v", codes, err)
	}
	for _, code := range codes {
		eventHub.closeBin(code)
	}
	if _, open := <-stream; open {
		t.Error("expired stream still open")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE bin_code = ?`, bin.Code).Scan(&count); err != nil || count != 0 {
		t.Errorf("orphan count = %d, %v", count, err)
	}
}

func TestSmallStoreCapRejectsCreationAndCapture(t *testing.T) {
	s := useTestStore(t)
	bin, _, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	var pageSize, pages int64
	if err := s.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if err := s.configureCapacity((pages + 1) * pageSize); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.createOwnedBin(); !errors.Is(err, ErrStoreFull) {
		t.Errorf("create = %v", err)
	}
	if _, err := s.saveRequest(ParsedRequest{RawBody: []byte(strings.Repeat("x", 2*int(pageSize)))}, bin.Code); !errors.Is(err, ErrStoreFull) {
		t.Errorf("capture = %v", err)
	}
	capture := callInspector(t, "POST", "/b/"+bin.Code, "x", "")
	if capture.Code != http.StatusInsufficientStorage || capture.Header().Get("X-Hooklook-Error") != "store_full" || !strings.Contains(capture.Body.String(), "global storage limit reached") {
		t.Errorf("global capacity response = %d, %q, %q", capture.Code, capture.Header().Get("X-Hooklook-Error"), capture.Body.String())
	}
	response := callInspector(t, "GET", "/", "", "")
	if response.Code != http.StatusInsufficientStorage || len(response.Result().Cookies()) != 0 {
		t.Errorf("home = %d, cookies %d", response.Code, len(response.Result().Cookies()))
	}
}

func TestBinMetadataReportsGlobalStoreFull(t *testing.T) {
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	var pageSize, pages int64
	if err := s.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if err := s.configureCapacity((pages + 1) * pageSize); err != nil {
		t.Fatal(err)
	}
	response := callInspector(t, "GET", "/api/bins/"+bin.Code, "", owner)
	if response.Code != http.StatusOK {
		t.Fatalf("metadata status = %d: %s", response.Code, response.Body.String())
	}
	var access BinAccess
	if err := json.Unmarshal(response.Body.Bytes(), &access); err != nil {
		t.Fatal(err)
	}
	global := access.StoreCapacity
	if !global.Full || access.Capacity.Full || global.MaxBytes != (pages+1)*pageSize || global.DatabaseBytes != pages*pageSize || global.ReusableBytes != 0 || global.AvailableBytes != pageSize {
		t.Errorf("global capacity = %+v, bin capacity = %+v", global, access.Capacity)
	}
}

func TestStartupWithDatabaseAlreadyAboveMaxStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooklook.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	initial := newBinStore(db)
	bin, owner, err := initial.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	id, err := initial.saveRequest(ParsedRequest{RawBody: []byte(strings.Repeat("x", 20_000))}, bin.Code)
	if err != nil {
		t.Fatal(err)
	}
	var pageSize, pages int64
	if err := db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAX_STORE", strconv.FormatInt((pages-1)*pageSize, 10))

	reopened, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := migrate(reopened); err != nil {
		t.Fatal(err)
	}
	configured, err := configureStore(reopened)
	if err != nil {
		t.Fatalf("restart above cap: %v", err)
	}
	originalStore, originalURL, originalHub := store, publicBaseURL, eventHub
	store, publicBaseURL, eventHub = configured, "https://hooklook.example", newEventHub()
	t.Cleanup(func() { store, publicBaseURL, eventHub = originalStore, originalURL, originalHub })

	info := callInspector(t, "GET", "/api/bins/"+bin.Code, "", owner)
	if info.Code != http.StatusOK {
		t.Fatalf("existing bin metadata = %d: %s", info.Code, info.Body.String())
	}
	var access BinAccess
	if err := json.Unmarshal(info.Body.Bytes(), &access); err != nil {
		t.Fatal(err)
	}
	if !access.StoreCapacity.Full || access.StoreCapacity.DatabaseBytes <= access.StoreCapacity.MaxBytes {
		t.Errorf("over-cap capacity = %+v", access.StoreCapacity)
	}
	if home := callInspector(t, "GET", "/", "", ""); home.Code != http.StatusInsufficientStorage || len(home.Result().Cookies()) != 0 {
		t.Errorf("new visitor home = %d, cookies %d", home.Code, len(home.Result().Cookies()))
	}
	if capture := callInspector(t, "POST", "/b/"+bin.Code, "new", ""); capture.Code != http.StatusInsufficientStorage || capture.Header().Get("X-Hooklook-Error") != "store_full" {
		t.Errorf("capture above cap = %d, %q", capture.Code, capture.Header().Get("X-Hooklook-Error"))
	}
	if deleted := callInspector(t, "DELETE", "/api/bins/"+bin.Code+"/requests/"+id, "", owner); deleted.Code != http.StatusNoContent {
		t.Errorf("delete while over cap = %d: %s", deleted.Code, deleted.Body.String())
	}
}

func TestStoreCapacityCountsReusablePagesAfterDeletion(t *testing.T) {
	s := newTestStore(t)
	bin, err := s.createBin()
	if err != nil {
		t.Fatal(err)
	}
	var pageSize int64
	if err := s.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	id, err := s.saveRequest(ParsedRequest{RawBody: []byte(strings.Repeat("x", 20*int(pageSize)))}, bin.Code)
	if err != nil {
		t.Fatal(err)
	}
	var pages int64
	if err := s.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if err := s.configureCapacity(pages * pageSize); err != nil {
		t.Fatal(err)
	}
	before, err := s.storeCapacity()
	if err != nil {
		t.Fatal(err)
	}
	if !before.Full {
		t.Fatalf("store unexpectedly has room: %+v", before)
	}
	if err := s.deleteRequest(bin.Code, id); err != nil {
		t.Fatal(err)
	}
	after, err := s.storeCapacity()
	if err != nil {
		t.Fatal(err)
	}
	if after.Full || after.DatabaseBytes != before.DatabaseBytes || after.ReusableBytes == 0 || after.AvailableBytes <= before.AvailableBytes {
		t.Errorf("reusable pages not reported: before %+v, after %+v", before, after)
	}
}

func TestHomeStoreFullBootstrapsAppWithoutCookie(t *testing.T) {
	t.Setenv("FRONTEND_DEV", "1")
	s := useTestStore(t)
	var pageSize, pages int64
	if err := s.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if err := s.configureCapacity((pages + 1) * pageSize); err != nil {
		t.Fatal(err)
	}
	response := callInspector(t, "GET", "/", "", "")
	if response.Code != http.StatusInsufficientStorage || response.Header().Get("X-Hooklook-Error") != "store_full" || len(response.Result().Cookies()) != 0 {
		t.Fatalf("home = %d, error %q, cookies %d", response.Code, response.Header().Get("X-Hooklook-Error"), len(response.Result().Cookies()))
	}
	if !strings.Contains(response.Body.String(), `data-hooklook-startup="store_full"`) || !strings.Contains(response.Body.String(), `/src/main.ts`) {
		t.Errorf("capacity response does not bootstrap the app: %s", response.Body.String())
	}
}

func TestBuiltCapacityPageLoadsEmbeddedApp(t *testing.T) {
	requireBuiltFrontend(t)
	t.Setenv("FRONTEND_DEV", "")
	rec := httptest.NewRecorder()
	writeCapacityShell(rec)
	if rec.Code != http.StatusInsufficientStorage || rec.Header().Get("X-Hooklook-Error") != "store_full" {
		t.Fatalf("capacity response = %d, %q", rec.Code, rec.Header().Get("X-Hooklook-Error"))
	}
	if !strings.Contains(rec.Body.String(), `data-hooklook-startup="store_full"`) || !strings.Contains(rec.Body.String(), `/assets/`) {
		t.Errorf("built capacity page does not load app assets: %s", rec.Body.String())
	}
}

func TestBinFullResponseTakesPrecedenceOverGlobalStoreFull(t *testing.T) {
	s := useTestStore(t)
	bin := insertTestBin(t, s, "full-bin")
	if _, err := s.db.Exec(`UPDATE bins SET total_body_bytes = ? WHERE code = ?`, maxStoredBodyBytes, bin.Code); err != nil {
		t.Fatal(err)
	}
	var pageSize, pages int64
	if err := s.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if err := s.configureCapacity((pages + 1) * pageSize); err != nil {
		t.Fatal(err)
	}
	response := callInspector(t, "POST", "/b/"+bin.Code, "x", "")
	if response.Code != http.StatusInsufficientStorage || response.Header().Get("X-Hooklook-Error") != "bin_full" || !strings.Contains(response.Body.String(), "bin storage limit reached") {
		t.Errorf("bin capacity response = %d, %q, %q", response.Code, response.Header().Get("X-Hooklook-Error"), response.Body.String())
	}
}

func TestSQLitePageLimitAlsoMapsToStoreFull(t *testing.T) {
	s := newTestStore(t)
	bin, err := s.createBin()
	if err != nil {
		t.Fatal(err)
	}
	var pageSize, pages int64
	if err := s.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if err := s.configureCapacity((pages + 1) * pageSize); err != nil {
		t.Fatal(err)
	}
	// Bypass the advisory preflight to exercise SQLite's independent hard cap.
	s.maxStorePages = 0
	if _, err := s.saveRequest(ParsedRequest{RawBody: []byte(strings.Repeat("x", 3*int(pageSize)))}, bin.Code); !errors.Is(err, ErrStoreFull) {
		t.Errorf("SQLite full error = %v", err)
	}
}

func TestOnlineBackupRestoresRequest(t *testing.T) {
	s := newTestStore(t)
	bin, err := s.createBin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.saveRequest(ParsedRequest{RawBody: []byte("backed up")}, bin.Code); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup.db")
	var source string
	if err := s.db.QueryRow(`PRAGMA database_list`).Scan(new(int), new(string), &source); err != nil {
		t.Fatal(err)
	}
	if err := backupDatabase(source, backup); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(dir, "restored.db")
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(restored, data, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := openDB(restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var body string
	if err := db.QueryRow(`SELECT raw_body FROM requests WHERE bin_code = ?`, bin.Code).Scan(&body); err != nil || body != "backed up" {
		t.Errorf("restored body = %q, %v", body, err)
	}
}
