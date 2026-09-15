package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	health(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if body := rec.Body.String(); body != "OK, I'm healthy\n" {
		t.Errorf("body = %q, want %q", body, "OK, I'm healthy\n")
	}
}

func TestCreateBin(t *testing.T) {
	originalStore := store
	originalPublicBaseURL := publicBaseURL
	store = newBinStore()
	publicBaseURL = "https://hooklook.example"
	t.Cleanup(func() {
		store = originalStore
		publicBaseURL = originalPublicBaseURL
	})

	req := httptest.NewRequest(http.MethodPost, "/api/bins", nil)
	rec := httptest.NewRecorder()

	createBin(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var response struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code == "" {
		t.Fatal("response code is empty")
	}
	if want := publicBaseURL + "/b/" + response.Code; response.URL != want {
		t.Errorf("url = %q, want %q", response.URL, want)
	}
}

func TestPublicBaseURLFromEnvironment(t *testing.T) {
	t.Setenv(publicBaseURLEnvironmentVariable, "https://hooklook.example/")

	got, err := publicBaseURLFromEnvironment()
	if err != nil {
		t.Fatalf("read public base URL: %v", err)
	}
	if want := "https://hooklook.example"; got != want {
		t.Errorf("public base URL = %q, want %q", got, want)
	}
}

func TestPublicBaseURLFromEnvironmentRejectsInvalidValue(t *testing.T) {
	t.Setenv(publicBaseURLEnvironmentVariable, "not-a-url")

	if _, err := publicBaseURLFromEnvironment(); err == nil {
		t.Fatal("read public base URL error = nil, want an error")
	}
}
