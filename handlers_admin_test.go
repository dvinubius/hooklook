package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testAdminToken = "operator-test-token"

func useAdminTestStore(t *testing.T) *Store {
	t.Helper()
	store := useTestStore(t)
	originalAdminToken := adminToken
	adminToken = testAdminToken
	t.Cleanup(func() { adminToken = originalAdminToken })
	return store
}

func adminRequest(method, target string, body []byte) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	return req
}

func TestAdminRoutesRequireOperatorBearerToken(t *testing.T) {
	useAdminTestStore(t)
	for _, test := range []struct{ method, target string }{
		{http.MethodGet, "/admin/bins"},
		{http.MethodDelete, "/admin/bins/bin"},
		{http.MethodPost, "/admin/tokens"},
		{http.MethodGet, "/admin/tokens"},
		{http.MethodDelete, "/admin/tokens/token-id"},
	} {
		rec := httptest.NewRecorder()
		routes().ServeHTTP(rec, httptest.NewRequest(test.method, test.target, nil))
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s %s response = %d, %q", test.method, test.target, rec.Code, rec.Header().Get("WWW-Authenticate"))
		}
	}
}

func TestAdminCreationTokenLifecycle(t *testing.T) {
	useAdminTestStore(t)
	issueRec := httptest.NewRecorder()
	routes().ServeHTTP(issueRec, adminRequest(http.MethodPost, "/admin/tokens", []byte(`{"label":"CI","maxUses":2}`)))
	if issueRec.Code != http.StatusCreated {
		t.Fatalf("issue status = %d, want %d: %s", issueRec.Code, http.StatusCreated, issueRec.Body.String())
	}
	var issued struct {
		CreationToken
		Token string `json:"token"`
	}
	if err := json.NewDecoder(issueRec.Body).Decode(&issued); err != nil {
		t.Fatalf("decode issued token: %v", err)
	}
	if issued.ID == "" || issued.Token == "" || issued.Label != "CI" || issued.MaxUses != 2 || issued.UseCount != 0 || issueRec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("issued token = %#v, Cache-Control = %q", issued, issueRec.Header().Get("Cache-Control"))
	}

	listRec := httptest.NewRecorder()
	routes().ServeHTTP(listRec, adminRequest(http.MethodGet, "/admin/tokens", nil))
	var tokens []CreationToken
	if err := json.NewDecoder(listRec.Body).Decode(&tokens); err != nil || listRec.Code != http.StatusOK || len(tokens) != 1 || tokens[0].ID != issued.ID {
		t.Fatalf("listed tokens = %#v, status = %d, error = %v", tokens, listRec.Code, err)
	}

	revokeRec := httptest.NewRecorder()
	routes().ServeHTTP(revokeRec, adminRequest(http.MethodDelete, "/admin/tokens/"+issued.ID, nil))
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, want %d", revokeRec.Code, http.StatusNoContent)
	}

	listRec = httptest.NewRecorder()
	routes().ServeHTTP(listRec, adminRequest(http.MethodGet, "/admin/tokens", nil))
	if err := json.NewDecoder(listRec.Body).Decode(&tokens); err != nil || len(tokens) != 1 || tokens[0].RevokedAt == nil {
		t.Errorf("revoked tokens = %#v, error = %v", tokens, err)
	}
}

func TestAdminListsAndDeletesBinWithCapturedRequests(t *testing.T) {
	store := useAdminTestStore(t)
	insertTestBin(t, store, "first-bin")
	if _, err := store.saveRequest(ParsedRequest{RawBody: []byte{}}, "first-bin"); err != nil {
		t.Fatalf("save request: %v", err)
	}

	listRec := httptest.NewRecorder()
	routes().ServeHTTP(listRec, adminRequest(http.MethodGet, "/admin/bins", nil))
	var bins []BinSummary
	if err := json.NewDecoder(listRec.Body).Decode(&bins); err != nil || listRec.Code != http.StatusOK || len(bins) != 1 || bins[0].Code != "first-bin" || bins[0].RequestCount != 1 {
		t.Fatalf("listed bins = %#v, status = %d, error = %v", bins, listRec.Code, err)
	}

	deleteRec := httptest.NewRecorder()
	routes().ServeHTTP(deleteRec, adminRequest(http.MethodDelete, "/admin/bins/first-bin", nil))
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d: %s", deleteRec.Code, http.StatusNoContent, deleteRec.Body.String())
	}

	var binCount, requestCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM bins`).Scan(&binCount); err != nil {
		t.Fatalf("count bins: %v", err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&requestCount); err != nil {
		t.Fatalf("count requests: %v", err)
	}
	if binCount != 0 || requestCount != 0 {
		t.Errorf("remaining rows = %d bins, %d requests; want none", binCount, requestCount)
	}
}
