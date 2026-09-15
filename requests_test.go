package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRequestCapturesFaithfulRequestFields(t *testing.T) {
	body := []byte{0x00, 0xff, 'h', 'i'}
	req := httptest.NewRequest(http.MethodPost, "/b/test-bin/github/events?tag=a%2Fb&tag=&enabled", bytes.NewReader(body))
	req.Header["content-type"] = []string{"application/octet-stream"}
	req.Header["authorization"] = []string{"Bearer first", "Bearer second"}
	req.Header["X-Trace-ID"] = []string{"first", "second"}

	parsed, err := parseRequest(req, "/github/events")
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}
	if parsed.Path != "/github/events" {
		t.Errorf("path = %q, want %q", parsed.Path, "/github/events")
	}
	if parsed.RawQuery != "tag=a%2Fb&tag=&enabled" {
		t.Errorf("raw query = %q, want %q", parsed.RawQuery, "tag=a%2Fb&tag=&enabled")
	}
	if !bytes.Equal(parsed.RawBody, body) {
		t.Errorf("raw body = %v, want %v", parsed.RawBody, body)
	}
	if parsed.ContentType != "application/octet-stream" {
		t.Errorf("content type = %q, want %q", parsed.ContentType, "application/octet-stream")
	}

	if got, want := parsed.Headers["authorization"], []string{redactedValue, redactedValue}; !equalStrings(got, want) {
		t.Errorf("authorization header = %q, want %q", got, want)
	}
	if got, want := parsed.Headers["X-Trace-ID"], []string{"first", "second"}; !equalStrings(got, want) {
		t.Errorf("trace header = %q, want %q", got, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
