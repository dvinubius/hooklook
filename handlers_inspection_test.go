package main

import (
	"encoding/json"
	"io"
	"net/http"
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

func TestHomeCreatesAndReusesCookieBin(t *testing.T) {
	s := useTestStore(t)
	first := callInspector(t, "GET", "/", "", "")
	if first.Code != http.StatusSeeOther || len(first.Result().Cookies()) != 1 {
		t.Fatalf("first home: %d %v", first.Code, first.Result().Cookies())
	}
	cookie := first.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags: %#v", cookie)
	}
	code := strings.TrimPrefix(first.Header().Get("Location"), "/bins/")
	if _, err := s.ownedBin(cookie.Value); err != nil {
		t.Fatalf("owner lookup: %v", err)
	}
	second := callInspector(t, "GET", "/", "", cookie.Value)
	if second.Header().Get("Location") != first.Header().Get("Location") || len(second.Result().Cookies()) != 0 {
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
	if got := callInspector(t, "GET", guestPath, "", "").Code; got != 404 {
		t.Errorf("disabled guest list = %d", got)
	}
	enabled := callInspector(t, "PUT", "/api/bins/"+bin.Code+"/sharing", `{"enabled":true}`, owner)
	if enabled.Code != 204 {
		t.Fatalf("enable sharing = %d %s", enabled.Code, enabled.Body.String())
	}
	if got := callInspector(t, "GET", guestPath, "", "").Code; got != 200 {
		t.Errorf("guest list = %d", got)
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
	if got := callInspector(t, "GET", guestPath, "", "").Code; got != 404 {
		t.Errorf("revoked guest list = %d", got)
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
