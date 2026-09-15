package main

import "testing"

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
