package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

func TestMaxRequestBodyBytesFromEnvironmentDefaultsTo256KiB(t *testing.T) {
	t.Setenv(maxRequestBodyBytesEnvironmentVariable, "")

	got, err := maxRequestBodyBytesFromEnvironment()
	if err != nil {
		t.Fatalf("read maximum request body bytes: %v", err)
	}
	if got != defaultMaxRequestBodyBytes {
		t.Errorf("maximum request body bytes = %d, want %d", got, defaultMaxRequestBodyBytes)
	}
}

func TestNewHTTPServerSetsLimitsAndTimeouts(t *testing.T) {
	server := newHTTPServer(":8080", http.HandlerFunc(health))

	if server.MaxHeaderBytes != defaultMaxRequestHeaderBytes {
		t.Errorf("maximum request header bytes = %d, want %d", server.MaxHeaderBytes, defaultMaxRequestHeaderBytes)
	}
	if server.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("read header timeout = %s, want %s", server.ReadHeaderTimeout, 5*time.Second)
	}
	if server.ReadTimeout != 15*time.Second {
		t.Errorf("read timeout = %s, want %s", server.ReadTimeout, 15*time.Second)
	}
	if server.IdleTimeout != time.Minute {
		t.Errorf("idle timeout = %s, want %s", server.IdleTimeout, time.Minute)
	}
	if server.WriteTimeout != 0 {
		t.Errorf("write timeout = %s, want 0", server.WriteTimeout)
	}
}

func TestHTTPServerRejectsOversizedHeaders(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(health))
	server.Config.MaxHeaderBytes = defaultMaxRequestHeaderBytes
	server.Start()
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("X-Large", strings.Repeat("a", defaultMaxRequestHeaderBytes*2))

	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Errorf("status = %d, want %d", response.StatusCode, http.StatusRequestHeaderFieldsTooLarge)
	}
}

func TestMaxRequestBodyBytesFromEnvironmentReadsConfiguredValue(t *testing.T) {
	t.Setenv(maxRequestBodyBytesEnvironmentVariable, "2048")

	got, err := maxRequestBodyBytesFromEnvironment()
	if err != nil {
		t.Fatalf("read maximum request body bytes: %v", err)
	}
	if got != 2048 {
		t.Errorf("maximum request body bytes = %d, want %d", got, 2048)
	}
}

func TestMaxRequestBodyBytesFromEnvironmentRejectsInvalidValue(t *testing.T) {
	t.Setenv(maxRequestBodyBytesEnvironmentVariable, "0")

	if _, err := maxRequestBodyBytesFromEnvironment(); err == nil {
		t.Fatal("read maximum request body bytes error = nil, want an error")
	}
}

func TestPublicBaseURLFromEnvironmentRejectsInvalidValue(t *testing.T) {
	t.Setenv(publicBaseURLEnvironmentVariable, "not-a-url")

	if _, err := publicBaseURLFromEnvironment(); err == nil {
		t.Fatal("read public base URL error = nil, want an error")
	}
}
