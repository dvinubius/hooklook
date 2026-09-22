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
	"time"
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

func TestUnavailableBinPagesExplainBeforeCreatingAReplacement(t *testing.T) {
	t.Setenv(frontendDevEnvironmentVariable, "1")
	s := useTestStore(t)
	shared, sharedOwner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.setSharing(shared.Code, true); err != nil {
		t.Fatal(err)
	}
	var sharedAccess BinAccess
	if err := json.Unmarshal(get(t, "/api/bins/"+shared.Code, sharedOwner).Body.Bytes(), &sharedAccess); err != nil {
		t.Fatal(err)
	}
	if err := s.setSharing(shared.Code, false); err != nil {
		t.Fatal(err)
	}

	expired, expiredOwner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.setSharing(expired.Code, true); err != nil {
		t.Fatal(err)
	}
	var expiredAccess BinAccess
	if err := json.Unmarshal(get(t, "/api/bins/"+expired.Code, expiredOwner).Body.Bytes(), &expiredAccess); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE bins SET expires_at = ? WHERE code = ?`, time.Now().Add(-time.Second).Unix(), expired.Code); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name, path, owner, state string
	}{
		{name: "revoked guest", path: "/bins/" + shared.Code + "/requests/7?invite=" + sharedAccess.InviteID, state: "shared_bin_unavailable"},
		{name: "invalid access", path: "/bins/" + shared.Code + "?invite=guessed", state: "shared_bin_unavailable"},
		{name: "active regular target without access", path: "/bins/" + shared.Code, state: "bin_expired"},
		{name: "missing guest target", path: "/bins/no-such-bin?invite=old-invitation", state: "shared_bin_unavailable"},
		{name: "expired owner target", path: "/bins/" + expired.Code, owner: expiredOwner, state: "bin_expired"},
		{name: "expired guest target", path: "/bins/" + expired.Code + "?invite=" + expiredAccess.InviteID, state: "shared_bin_unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			page := get(t, test.path, test.owner)
			if page.Code != http.StatusNotFound || page.Header().Get("X-Hooklook-Error") != test.state {
				t.Fatalf("status=%d error=%q", page.Code, page.Header().Get("X-Hooklook-Error"))
			}
			if !strings.Contains(page.Body.String(), `data-hooklook-startup="`+test.state+`"`) {
				t.Fatalf("page document has no %q startup marker", test.state)
			}
			if page.Header().Get("Location") != "" || len(page.Result().Cookies()) != 0 {
				t.Errorf("page redirected or replaced the bin: location=%q cookies=%v", page.Header().Get("Location"), page.Result().Cookies())
			}
		})
	}
}

func TestPageURLsWithATrailingSlashRedirectToTheCanonicalOnes(t *testing.T) {
	useTestStore(t)
	for path, want := range map[string]string{
		"/bins/keen-canyon-30/":                     "/bins/keen-canyon-30",
		"/bins/keen-canyon-30/?invite=abc":          "/bins/keen-canyon-30?invite=abc",
		"/bins/keen-canyon-30/requests/7/":          "/bins/keen-canyon-30/requests/7",
		"/bins/keen-canyon-30/requests/7/?invite=x": "/bins/keen-canyon-30/requests/7?invite=x",
	} {
		page := get(t, path, "")
		if page.Code != http.StatusMovedPermanently || page.Header().Get("Location") != want {
			t.Errorf("GET %s = %d → %q, want %d → %q",
				path, page.Code, page.Header().Get("Location"), http.StatusMovedPermanently, want)
		}
	}
}

// A detail URL with its request id cut off is the bin page, not a 404.
func TestTheRequestsPathWithoutAnIdRedirectsToTheBinPage(t *testing.T) {
	useTestStore(t)
	for path, want := range map[string]string{
		"/bins/keen-canyon-30/requests":            "/bins/keen-canyon-30",
		"/bins/keen-canyon-30/requests/":           "/bins/keen-canyon-30",
		"/bins/keen-canyon-30/requests/?invite=ab": "/bins/keen-canyon-30?invite=ab",
	} {
		page := get(t, path, "")
		if page.Code != http.StatusMovedPermanently || page.Header().Get("Location") != want {
			t.Errorf("GET %s = %d → %q, want %d → %q",
				path, page.Code, page.Header().Get("Location"), http.StatusMovedPermanently, want)
		}
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
		// Only the hashed build output may be cached permanently; anything the
		// document names by a stable path — today just the favicon — takes the
		// ordinary lifetime, so a new one can replace it.
		wantCache := hashedAssetCache
		if !strings.HasPrefix(path, "/assets/") {
			wantCache = staticFileCache
		}
		if got := asset.Header().Get("Cache-Control"); got != wantCache {
			t.Errorf("%s cache control = %q, want %q", path, got, wantCache)
		}
		wantType := map[string]string{
			".js":  "text/javascript",
			".css": "text/css",
			".svg": "image/svg+xml",
		}[path[strings.LastIndex(path, "."):]]
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

// The favicon sits at the build root rather than under /assets, so it needs a
// route of its own; a browser asks for it without ever being handed a bin.
func TestFaviconIsServedWithoutBinAuthorization(t *testing.T) {
	requireBuiltFrontend(t)

	icon := get(t, "/favicon.svg", "")
	if icon.Code != http.StatusOK {
		t.Fatalf("/favicon.svg = %d", icon.Code)
	}
	if got := icon.Header().Get("Content-Type"); !strings.HasPrefix(got, "image/svg+xml") {
		t.Errorf("favicon content type = %q", got)
	}
	if got := icon.Header().Get("Cache-Control"); got != staticFileCache {
		t.Errorf("favicon cache control = %q", got)
	}
}

func TestMissingAssetsAreNotFound(t *testing.T) {
	for _, path := range []string{"/assets/index-deadbeef.js", "/assets/", "/fonts/absent.woff2"} {
		if got := get(t, path, "").Code; got != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, got)
		}
	}
}
