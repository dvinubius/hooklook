package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func callInspector(t *testing.T, method, path, body, owner string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if owner != "" {
		req.AddCookie(&http.Cookie{Name: ownerCookieName, Value: owner})
	}
	if method != http.MethodGet {
		req.Header.Set("Origin", "https://hooklook.example")
	}
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)
	return rec
}

// cookieCheck is the probe a client with a cookie jar returns on the second
// hop through `/`.
var cookieCheck = &http.Cookie{Name: cookieCheckName, Value: "1"}

// callHome sends one GET through `/` with the cookies a client would carry on
// that hop.
func callHome(t *testing.T, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)
	return rec
}

func responseCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestHomeChecksCookiesBeforeCreatingABin(t *testing.T) {
	s := useTestStore(t)
	probe := callHome(t, "/")
	if probe.Code != http.StatusSeeOther || probe.Header().Get("Location") != "/?cookie-check" {
		t.Fatalf("first hop: %d %q", probe.Code, probe.Header().Get("Location"))
	}
	check := responseCookie(probe, cookieCheckName)
	if check == nil || check.MaxAge != 60 || !check.HttpOnly || !check.Secure || check.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie check: %#v", check)
	}
	if responseCookie(probe, ownerCookieName) != nil {
		t.Error("first hop set an owner cookie")
	}
	if bins, err := s.getAllBins(); err != nil || len(bins) != 0 {
		t.Fatalf("first hop created %d bins (%v)", len(bins), err)
	}

	first := callHome(t, "/?cookie-check", check)
	if first.Code != http.StatusSeeOther || !strings.HasPrefix(first.Header().Get("Location"), "/bins/") {
		t.Fatalf("second hop: %d %q", first.Code, first.Header().Get("Location"))
	}
	if cleared := responseCookie(first, cookieCheckName); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("cookie check was not cleared: %#v", cleared)
	}
	cookie := responseCookie(first, ownerCookieName)
	if cookie == nil || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("owner cookie: %#v", cookie)
	}
	code := strings.TrimPrefix(first.Header().Get("Location"), "/bins/")
	if _, err := s.ownedBin(cookie.Value); err != nil {
		t.Fatalf("owner lookup: %v", err)
	}

	second := callInspector(t, "GET", "/", "", cookie.Value)
	if second.Header().Get("Location") != first.Header().Get("Location") || len(second.Result().Cookies()) != 1 {
		t.Errorf("repeat home: %d %s", second.Code, second.Header().Get("Location"))
	}
	outsider := callInspector(t, "GET", "/api/bins/"+code+"/requests", "", "")
	if outsider.Code != 404 {
		t.Errorf("public list status = %d", outsider.Code)
	}
	owner := callInspector(t, "GET", "/api/bins/"+code+"/requests", "", cookie.Value)
	if owner.Code != 200 {
		t.Errorf("owner list status = %d", owner.Code)
	}
}

// A client that drops cookies is told so on the marked second hop, and not
// sent round again.
func TestHomeCreatesNoBinForAClientThatDropsCookies(t *testing.T) {
	t.Setenv(frontendDevEnvironmentVariable, "1")
	s := useTestStore(t)
	original := telemetry
	telemetry = newTelemetry()
	t.Cleanup(func() { telemetry = original })

	refused := callHome(t, "/?cookie-check")
	if refused.Code != http.StatusOK || refused.Header().Get("X-Hooklook-Error") != "cookies_required" {
		t.Fatalf("second hop without the check = %d, error %q", refused.Code, refused.Header().Get("X-Hooklook-Error"))
	}
	if refused.Header().Get("Location") != "" || len(refused.Result().Cookies()) != 0 {
		t.Errorf("refusal redirected or set cookies: %q %v", refused.Header().Get("Location"), refused.Result().Cookies())
	}
	if !strings.Contains(refused.Body.String(), `data-hooklook-startup="cookies_required"`) {
		t.Errorf("refusal does not bootstrap the app: %s", refused.Body.String())
	}
	if bins, err := s.getAllBins(); err != nil || len(bins) != 0 {
		t.Fatalf("refusal created %d bins (%v)", len(bins), err)
	}

	metrics := httptest.NewRecorder()
	metricsHandler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(metrics.Body.String(), `hooklook_bin_creation_results_total{result="no_cookie"} 1`) {
		t.Error("refusal was not counted as no_cookie")
	}
}

// Holding the cookie of an expired bin is not the cookie check: the visitor
// goes through the same round-trip as a first-time one.
func TestHomeChecksCookiesAgainForAStaleOwnerCookie(t *testing.T) {
	s := useTestStore(t)
	response := callInspector(t, "GET", "/", "", "stale-owner-secret")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/?cookie-check" {
		t.Fatalf("stale owner home: %d %q", response.Code, response.Header().Get("Location"))
	}
	if bins, err := s.getAllBins(); err != nil || len(bins) != 0 {
		t.Fatalf("stale owner created %d bins (%v)", len(bins), err)
	}
}

// The whole flow has to work for a plain HTTP client that keeps cookies, such
// as the synthetic traffic generator, and end without a bin for one that does
// not.
func TestHomeFlowThroughAnHTTPClient(t *testing.T) {
	t.Setenv(frontendDevEnvironmentVariable, "1")
	s := useTestStore(t)
	server := httptest.NewTLSServer(routes())
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	withJar := &http.Client{Transport: server.Client().Transport, Jar: jar}
	page, err := withJar.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	page.Body.Close()
	if page.StatusCode != http.StatusOK || !strings.HasPrefix(page.Request.URL.Path, "/bins/") {
		t.Fatalf("client with a jar ended at %d %s", page.StatusCode, page.Request.URL)
	}

	withoutJar := &http.Client{Transport: server.Client().Transport}
	refused, err := withoutJar.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	refused.Body.Close()
	if refused.StatusCode != http.StatusOK || refused.Header.Get("X-Hooklook-Error") != "cookies_required" {
		t.Fatalf("client without a jar ended at %d %s, error %q", refused.StatusCode, refused.Request.URL, refused.Header.Get("X-Hooklook-Error"))
	}

	if bins, err := s.getAllBins(); err != nil || len(bins) != 1 {
		t.Errorf("bins created = %d (%v), want 1", len(bins), err)
	}
}

// Link-preview scrapers drop the owner cookie between the redirect and the
// bin page, so they would land on a 404. They get the document with its
// preview tags instead, and no bin is created for them.
func TestHomeServesLinkPreviewScrapersWithoutCreatingABin(t *testing.T) {
	requireBuiltFrontend(t)
	s := useTestStore(t)
	for _, agent := range []string{
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
		"LinkedInBot/1.0 (compatible; Mozilla/5.0; Apache-HttpClient +http://www.linkedin.com)",
		"Twitterbot/1.0",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("User-Agent", agent)
		rec := httptest.NewRecorder()
		routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", agent, rec.Code)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s: set a cookie", agent)
		}
		if !strings.Contains(rec.Body.String(), `property="og:image"`) {
			t.Errorf("%s: document has no og:image tag", agent)
		}
	}
	bins, err := s.getAllBins()
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) != 0 {
		t.Errorf("scrapers created %d bins", len(bins))
	}
}

func TestBinInfoReportsCapacityAndReopensAfterDeletion(t *testing.T) {
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	read := func() BinAccess {
		t.Helper()
		response := callInspector(t, "GET", "/api/bins/"+bin.Code, "", owner)
		if response.Code != http.StatusOK {
			t.Fatalf("bin info status = %d", response.Code)
		}
		var access BinAccess
		if err := json.Unmarshal(response.Body.Bytes(), &access); err != nil {
			t.Fatal(err)
		}
		return access
	}
	initialAccess := read()
	initial := initialAccess.Capacity
	if initial.Full || initial.RequestCount != 0 || initial.RequestLimit != maxStoredRequests || initial.BodyBytesLimit != maxStoredBodyBytes {
		t.Fatalf("initial capacity = %+v", initial)
	}
	if initialAccess.StoreCapacity.Full || initialAccess.StoreCapacity.MaxBytes == 0 || initialAccess.StoreCapacity.DatabaseBytes == 0 || initialAccess.StoreCapacity.AvailableBytes == 0 {
		t.Fatalf("initial store capacity = %+v", initialAccess.StoreCapacity)
	}
	for range maxStoredRequests {
		if _, err := s.saveRequest(ParsedRequest{RawBody: []byte("x")}, bin.Code); err != nil {
			t.Fatal(err)
		}
	}
	filled := read().Capacity
	if !filled.Full || !filled.RequestsFull || filled.BodyBytesFull || filled.RequestCount != maxStoredRequests || filled.BodyBytesUsed != maxStoredRequests {
		t.Errorf("filled capacity = %+v", filled)
	}
	if err := s.deleteRequest(bin.Code, "1"); err != nil {
		t.Fatal(err)
	}
	reopened := read().Capacity
	if reopened.Full || reopened.RequestCount != maxStoredRequests-1 || reopened.BodyBytesUsed != maxStoredRequests-1 {
		t.Errorf("capacity after delete = %+v", reopened)
	}
	if _, err := s.db.Exec(`UPDATE bins SET total_body_bytes = ? WHERE code = ?`, maxStoredBodyBytes, bin.Code); err != nil {
		t.Fatal(err)
	}
	bodyFilled := read().Capacity
	if !bodyFilled.Full || bodyFilled.RequestsFull || !bodyFilled.BodyBytesFull || bodyFilled.BodyBytesUsed != maxStoredBodyBytes {
		t.Errorf("body-byte capacity = %+v", bodyFilled)
	}
}

func TestGuestAccessAndOwnerMutations(t *testing.T) {
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.saveRequest(ParsedRequest{Method: POST, Headers: HeaderMap{"X-Event": {"push"}}, RawBody: []byte("hello"), BodySizeKiB: 1}, bin.Code)
	if err != nil {
		t.Fatal(err)
	}
	info := callInspector(t, "GET", "/api/bins/"+bin.Code, "", owner)
	var access BinAccess
	if err := json.Unmarshal(info.Body.Bytes(), &access); err != nil || !access.Owner || access.InviteID == "" {
		t.Fatalf("owner info: %d %#v %v", info.Code, access, err)
	}
	guestPath := "/api/bins/" + bin.Code + "/requests?invite=" + access.InviteID
	disabledGuest := callInspector(t, "GET", guestPath, "", "")
	if disabledGuest.Code != 404 || disabledGuest.Header().Get("X-Hooklook-Error") != "shared_bin_unavailable" {
		t.Errorf("disabled guest list = %d, error = %q", disabledGuest.Code, disabledGuest.Header().Get("X-Hooklook-Error"))
	}
	enabled := callInspector(t, "PUT", "/api/bins/"+bin.Code+"/sharing", `{"enabled":true}`, owner)
	if enabled.Code != 204 {
		t.Fatalf("enable sharing = %d %s", enabled.Code, enabled.Body.String())
	}
	if got := callInspector(t, "GET", guestPath, "", "").Code; got != 200 {
		t.Errorf("guest list = %d", got)
	}
	guestInfo := callInspector(t, "GET", "/api/bins/"+bin.Code+"?invite="+access.InviteID, "", "")
	var guestAccess BinAccess
	if err := json.Unmarshal(guestInfo.Body.Bytes(), &guestAccess); err != nil || guestInfo.Code != 200 || guestAccess.Owner || guestAccess.Capacity.RequestCount != 1 || guestAccess.Capacity.BodyBytesUsed != 5 || guestAccess.StoreCapacity.MaxBytes == 0 {
		t.Errorf("guest capacity = %d %#v %v", guestInfo.Code, guestAccess.Capacity, err)
	}
	detail := callInspector(t, "GET", "/api/bins/"+bin.Code+"/requests/"+id+"?invite="+access.InviteID, "", "")
	var request RequestDetail
	if err := json.Unmarshal(detail.Body.Bytes(), &request); err != nil || string(request.RawBody) != "hello" || request.HeaderCount != 1 {
		t.Errorf("detail: %d %#v %v", detail.Code, request, err)
	}
	if got := callInspector(t, "DELETE", "/api/bins/"+bin.Code+"/requests/"+id+"?invite="+access.InviteID, "", "").Code; got != 403 {
		t.Errorf("guest delete = %d", got)
	}
	if got := callInspector(t, "DELETE", "/api/bins/"+bin.Code+"/requests/"+id, "", owner).Code; got != 204 {
		t.Errorf("owner delete = %d", got)
	}
	after, err := s.ownedBin(owner)
	if err != nil || after.TotalBodyBytes != 0 {
		t.Errorf("bytes after deletion: %#v %v", after, err)
	}
	if got := callInspector(t, "GET", "/api/bins/"+bin.Code+"/requests/"+id+"?invite="+access.InviteID, "", "").Code; got != 404 {
		t.Errorf("deleted detail = %d", got)
	}
	if got := callInspector(t, "PUT", "/api/bins/"+bin.Code+"/sharing", `{"enabled":false}`, owner).Code; got != 204 {
		t.Errorf("disable sharing = %d", got)
	}
	revokedGuest := callInspector(t, "GET", guestPath, "", "")
	if revokedGuest.Code != 404 || revokedGuest.Header().Get("X-Hooklook-Error") != "shared_bin_unavailable" {
		t.Errorf("revoked guest list = %d, error = %q", revokedGuest.Code, revokedGuest.Header().Get("X-Hooklook-Error"))
	}
}

func TestSharingUpdateAcceptsLargeValidJSON(t *testing.T) {
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}

	body := strings.Repeat(" ", 2048) + `{"enabled":true}`
	response := callInspector(t, http.MethodPut, "/api/bins/"+bin.Code+"/sharing", body, owner)
	if response.Code != http.StatusNoContent {
		t.Fatalf("sharing update = %d: %s", response.Code, response.Body.String())
	}

	info := callInspector(t, http.MethodGet, "/api/bins/"+bin.Code, "", owner)
	var access BinAccess
	if err := json.Unmarshal(info.Body.Bytes(), &access); err != nil || !access.SharingEnabled {
		t.Fatalf("sharing state after update = %d, enabled %t, error %v", info.Code, access.SharingEnabled, err)
	}
}

func TestBinReplacementRouteDoesNotExist(t *testing.T) {
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	response := callInspector(t, "POST", "/api/bins/"+bin.Code+"/replace", "", owner)
	if response.Code != http.StatusNotFound {
		t.Errorf("replacement route status = %d, want 404", response.Code)
	}
	if _, err := s.ownedBin(owner); err != nil {
		t.Errorf("original bin was changed: %v", err)
	}
}

func TestMutationRejectsMissingOrigin(t *testing.T) {
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("DELETE", "/api/bins/"+bin.Code+"/requests", nil)
	req.AddCookie(&http.Cookie{Name: ownerCookieName, Value: owner})
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("missing origin = %d", rec.Code)
	}
}

func TestDisablingSharingClosesGuestStream(t *testing.T) {
	s := useTestStore(t)
	bin, owner, err := s.createOwnedBin()
	if err != nil {
		t.Fatal(err)
	}
	info := callInspector(t, "GET", "/api/bins/"+bin.Code, "", owner)
	var access BinAccess
	if err := json.Unmarshal(info.Body.Bytes(), &access); err != nil {
		t.Fatal(err)
	}
	if err := s.setSharing(bin.Code, true); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(routes())
	defer server.Close()
	stream, err := server.Client().Get(server.URL + "/api/bins/" + bin.Code + "/events?invite=" + access.InviteID)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if stream.StatusCode != 200 {
		t.Fatalf("guest stream status = %d", stream.StatusCode)
	}
	opening := make([]byte, len(": connected\n\n"))
	if _, err := io.ReadFull(stream.Body, opening); err != nil {
		t.Fatalf("read opening comment: %v", err)
	}
	if got := callInspector(t, "PUT", "/api/bins/"+bin.Code+"/sharing", `{"enabled":false}`, owner).Code; got != 204 {
		t.Fatalf("disable status = %d", got)
	}
	done := make(chan error, 1)
	go func() { var b [1]byte; _, err := stream.Body.Read(b[:]); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("stream remained open")
		}
	case <-time.After(2 * time.Second):
		t.Error("guest stream did not close")
	}
}
