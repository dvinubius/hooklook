package main

import (
	"net/http"
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

func TestNewHTTPServerSetsLimitsAndTimeouts(t *testing.T) {
	server := newHTTPServer(":8080", http.HandlerFunc(health))

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

func TestPublicBaseURLFromEnvironmentRejectsInvalidValue(t *testing.T) {
	t.Setenv(publicBaseURLEnvironmentVariable, "not-a-url")

	if _, err := publicBaseURLFromEnvironment(); err == nil {
		t.Fatal("read public base URL error = nil, want an error")
	}
}

func TestAdminTokenFromEnvironment(t *testing.T) {
	t.Setenv(adminTokenEnvironmentVariable, "")
	if _, err := adminTokenFromEnvironment(); err == nil {
		t.Fatal("missing admin token accepted")
	}
	t.Setenv(adminTokenEnvironmentVariable, "secret")
	if got, err := adminTokenFromEnvironment(); err != nil || got != "secret" {
		t.Errorf("admin token = %q, error = %v", got, err)
	}
}

func TestListenAddressFromEnvironmentDefaultsToLoopback(t *testing.T) {
	t.Setenv(listenAddressEnvironmentVariable, "")

	got, err := listenAddressFromEnvironment()
	if err != nil {
		t.Fatalf("read listen address: %v", err)
	}
	if got != defaultListenAddress {
		t.Errorf("listen address = %q, want %q", got, defaultListenAddress)
	}
}

func TestListenAddressFromEnvironmentAcceptsDockerAddress(t *testing.T) {
	t.Setenv(listenAddressEnvironmentVariable, "0.0.0.0:8080")

	got, err := listenAddressFromEnvironment()
	if err != nil {
		t.Fatalf("read listen address: %v", err)
	}
	if got != "0.0.0.0:8080" {
		t.Errorf("listen address = %q, want Docker address", got)
	}
}

func TestListenAddressFromEnvironmentRejectsInvalidValue(t *testing.T) {
	for _, address := range []string{
		":8080",
		"127.0.0.1",
		"127.0.0.1:not-a-port",
		"127.0.0.1:0",
		"127.0.0.1:65536",
	} {
		t.Run(address, func(t *testing.T) {
			t.Setenv(listenAddressEnvironmentVariable, address)
			if _, err := listenAddressFromEnvironment(); err == nil {
				t.Fatal("invalid listen address accepted")
			}
		})
	}
}

func TestCreateDevelopmentBin(t *testing.T) {
	t.Setenv(publicBaseURLEnvironmentVariable, "http://localhost:8080")
	path := t.TempDir() + "/hooklook.db"
	url, err := createDevelopmentBin(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "http://localhost:8080/b/") {
		t.Fatalf("development bin URL = %q", url)
	}
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bins`).Scan(&count); err != nil || count != 1 {
		t.Errorf("bin count = %d, error = %v", count, err)
	}
}
