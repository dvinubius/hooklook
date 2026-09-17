package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// requireBuiltFrontend skips tests that need real built assets. A fresh
// checkout embeds only the placeholder, so `go test ./...` still has to work
// before anyone has run the frontend build.
func requireBuiltFrontend(t *testing.T) {
	t.Helper()
	if prodShellErr != nil {
		t.Skip("no frontend build embedded; run `make build-web` to exercise this")
	}
}

func get(t *testing.T, path, owner string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if owner != "" {
		req.AddCookie(&http.Cookie{Name: ownerCookieName, Value: owner})
	}
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)
	return rec
}

func TestBinPagesServeTheApplicationShell(t *testing.T) {
	requireBuiltFrontend(t)
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}

	// The detail URL a capture reports, and an id that does not exist, both
	// get the same document: the page is authorized here, the request id is
	// resolved by the application afterwards.
	for _, path := range []string{
		"/bins/" + bin.Code,
		"/bins/" + bin.Code + "/requests/does-not-exist",
	} {
		page := get(t, path, owner)
		if page.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, page.Code, page.Body.String())
		}
		if got := page.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("%s content type = %q", path, got)
		}
		if page.Header().Get("Cache-Control") != "no-store" || page.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("%s headers = %v", path, page.Header())
		}
		if !bytes.Equal(page.Body.Bytes(), prodShell) {
			t.Errorf("%s did not serve the built document", path)
		}
	}
}

func TestGuestInvitationReachesBothPageURLs(t *testing.T) {
	requireBuiltFrontend(t)
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.setSharing(bin.Code, true); err != nil {
		t.Fatal(err)
	}
	var access BinAccess
	if err := json.Unmarshal(get(t, "/api/bins/"+bin.Code, owner).Body.Bytes(), &access); err != nil || access.InviteID == "" {
		t.Fatalf("owner metadata: %#v %v", access, err)
	}

	// An invited guest reaches the page and the detail URL with the invitation
	// in the query string, and is never handed an owner cookie of their own.
	for _, path := range []string{
		"/bins/" + bin.Code + "?invite=" + access.InviteID,
		"/bins/" + bin.Code + "/requests/some-id?invite=" + access.InviteID,
	} {
		page := get(t, path, "")
		if page.Code != http.StatusOK {
			t.Errorf("invited guest at %s = %d", path, page.Code)
		}
		if len(page.Result().Cookies()) != 0 {
			t.Errorf("%s gave the guest an owner cookie", path)
		}
	}
}

func TestUnauthorizedPageVisitRedirectsToTheVisitorsOwnBin(t *testing.T) {
	s := useTestStore(t)
	other, _, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}

	// A detail URL is as private as the bin page: an outsider is sent to their
	// own bin before any request data is loaded, and never to a foreign one.
	page := get(t, "/bins/"+other.Code+"/requests/anything?invite=guessed", "")
	if page.Code != http.StatusSeeOther {
		t.Fatalf("outsider at a detail URL = %d", page.Code)
	}
	location := page.Header().Get("Location")
	if strings.Contains(location, other.Code) || strings.Contains(location, "invite") {
		t.Errorf("redirect leaked the foreign bin: %q", location)
	}
	cookies := page.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("redirect set %d cookies, want the visitor's own", len(cookies))
	}
	own, err := s.ownedBin(cookies[0].Value)
	if err != nil || location != "/bins/"+own.Code {
		t.Errorf("redirect target = %q, own bin = %v (%v)", location, own.Code, err)
	}
}

var shellAssetReference = regexp.MustCompile(`(?:src|href)="(/[^"]+)"`)

// The built document references its assets by absolute path, which is what lets
// a visitor open /bins/{code} or a detail URL directly and reload it. Every one
// of those paths has to be answerable by this service.
func TestShellAssetsAreServedWithoutBinAuthorization(t *testing.T) {
	requireBuiltFrontend(t)

	references := shellAssetReference.FindAllStringSubmatch(string(prodShell), -1)
	if len(references) < 2 {
		t.Fatalf("built document referenced %d assets, want script and stylesheet", len(references))
	}
	for _, reference := range references {
		path := reference[1]
		// No cookie and no invitation: assets carry no captured data, and the
		// browser must be able to fetch them for a page it was just handed.
		asset := get(t, path, "")
		if asset.Code != http.StatusOK {
			t.Errorf("%s = %d", path, asset.Code)
			continue
		}
		if len(asset.Result().Cookies()) != 0 {
			t.Errorf("%s went through bin resolution and set a cookie", path)
		}
		if got := asset.Header().Get("Cache-Control"); got != hashedAssetCache {
			t.Errorf("%s cache control = %q", path, got)
		}
		wantType := map[string]string{".js": "text/javascript", ".css": "text/css"}[path[strings.LastIndex(path, "."):]]
		if wantType == "" {
			continue
		}
		if got := asset.Header().Get("Content-Type"); !strings.HasPrefix(got, wantType) {
			t.Errorf("%s content type = %q, want %s", path, got, wantType)
		}
	}
}

func TestFontsAreServedWithTheirOwnContentType(t *testing.T) {
	requireBuiltFrontend(t)

	fonts, err := fs.Glob(builtFrontend, "fonts/*.woff2")
	if err != nil || len(fonts) == 0 {
		t.Fatalf("built fonts = %v (%v)", fonts, err)
	}
	font := get(t, "/"+fonts[0], "")
	if font.Code != http.StatusOK {
		t.Fatalf("/%s = %d", fonts[0], font.Code)
	}
	if got := font.Header().Get("Content-Type"); got != "font/woff2" {
		t.Errorf("font content type = %q", got)
	}
	if got := font.Header().Get("Cache-Control"); got != staticFileCache {
		t.Errorf("font cache control = %q", got)
	}
}

func TestMissingAssetsAreNotFound(t *testing.T) {
	for _, path := range []string{"/assets/index-deadbeef.js", "/assets/", "/fonts/absent.woff2"} {
		if got := get(t, path, "").Code; got != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, got)
		}
	}
}
