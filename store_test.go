package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGenerateBinCode(t *testing.T) {
	for range 100 {
		code, err := generateCode(binCodeLength)
		if err != nil {
			t.Fatalf("generate bin code: %v", err)
		}
		if len(code) != binCodeLength {
			t.Fatalf("code length = %d, want %d", len(code), binCodeLength)
		}
		for _, character := range code {
			if !strings.ContainsRune(storeCodeAlphabet, character) {
				t.Fatalf("code %q contains character %q outside base-62 alphabet", code, character)
			}
		}
	}
}

func TestGenerateRequestCode(t *testing.T) {
	for range 100 {
		code, err := generateCode(requestCodeLength)
		if err != nil {
			t.Fatalf("generate bin code: %v", err)
		}
		if len(code) != requestCodeLength {
			t.Fatalf("code length = %d, want %d", len(code), requestCodeLength)
		}
		for _, character := range code {
			if !strings.ContainsRune(storeCodeAlphabet, character) {
				t.Fatalf("code %q contains character %q outside base-62 alphabet", code, character)
			}
		}
	}
}

func TestCreateRetriesCodeCollision(t *testing.T) {
	existingCode := strings.Repeat("A", binCodeLength)
	freshCode := strings.Repeat("B", binCodeLength)
	codes := []string{existingCode, freshCode}

	store := newBinStore()
	store.Bins = map[string]Bin{existingCode: {Code: existingCode}}
	store.generateCode = func(length int) (string, error) {
		code := codes[0]
		codes = codes[1:]
		return code, nil
	}

	bin, err := store.createBin()
	if err != nil {
		t.Fatalf("create bin: %v", err)
	}
	if bin.Code == existingCode {
		t.Fatal("created bin reused an existing code")
	}
	if bin.Code != freshCode {
		t.Errorf("code = %q, want %q", bin.Code, freshCode)
	}
}

func TestCreateReturnsCodeGenerationError(t *testing.T) {
	wantErr := errors.New("randomness unavailable")
	store := newBinStore()
	store.generateCode = func(length int) (string, error) {
		return "", wantErr
	}

	_, err := store.createBin()
	if !errors.Is(err, wantErr) {
		t.Fatalf("create bin error = %v, want wrapped %v", err, wantErr)
	}
}

func TestCreateStoresBinWithExpiry(t *testing.T) {
	store := newBinStore()

	bin, err := store.createBin()
	if err != nil {
		t.Fatalf("create bin: %v", err)
	}

	stored, ok := store.Bins[bin.Code]
	if !ok {
		t.Fatalf("bin %q was not stored", bin.Code)
	}
	if stored.Code != bin.Code {
		t.Errorf("stored code = %q, want %q", stored.Code, bin.Code)
	}
	if !stored.CreatedAt.Equal(bin.CreatedAt) {
		t.Errorf("stored creation time = %s, want %s", stored.CreatedAt, bin.CreatedAt)
	}
	if !stored.ExpiresAt.Equal(bin.ExpiresAt) {
		t.Errorf("stored expiry time = %s, want %s", stored.ExpiresAt, bin.ExpiresAt)
	}
	if stored.Requests == nil {
		t.Error("stored requests map is nil")
	}
	if got, want := bin.ExpiresAt.Sub(bin.CreatedAt), defaultBinTTL; got != want {
		t.Errorf("expiry duration = %s, want %s", got, want)
	}
	if bin.CreatedAt.Location() != time.UTC {
		t.Errorf("created at location = %s, want UTC", bin.CreatedAt.Location())
	}
}

func TestSaveRequestRetriesCodeCollision(t *testing.T) {
	binCode := strings.Repeat("A", binCodeLength)
	existingRequestCode := strings.Repeat("B", requestCodeLength)
	freshRequestCode := strings.Repeat("C", requestCodeLength)
	codes := []string{existingRequestCode, freshRequestCode}
	store := newBinStore()
	store.Bins[binCode] = Bin{
		Code: binCode,
		Requests: map[string]ParsedRequest{
			existingRequestCode: {Id: existingRequestCode},
		},
	}
	store.generateCode = func(length int) (string, error) {
		if length != requestCodeLength {
			t.Fatalf("code length = %d, want %d", length, requestCodeLength)
		}
		code := codes[0]
		codes = codes[1:]
		return code, nil
	}

	got, err := store.saveRequest(ParsedRequest{Method: POST}, binCode)
	if err != nil {
		t.Fatalf("save request: %v", err)
	}
	if got != freshRequestCode {
		t.Errorf("request code = %q, want %q", got, freshRequestCode)
	}
	if stored := store.Bins[binCode].Requests[freshRequestCode]; stored.Id != freshRequestCode {
		t.Errorf("stored request ID = %q, want %q", stored.Id, freshRequestCode)
	}
}

func TestSaveRequestRejectsUnknownBin(t *testing.T) {
	store := newBinStore()

	_, err := store.saveRequest(ParsedRequest{}, "missing")
	if !errors.Is(err, ErrBinNotFound) {
		t.Errorf("save request error = %v, want %v", err, ErrBinNotFound)
	}
}

func TestGetBinRequestsReturnsEmptySlice(t *testing.T) {
	store := newBinStore()
	store.Bins["bin"] = Bin{Code: "bin", Requests: make(map[string]ParsedRequest)}

	requests, err := store.getBinRequests("bin")
	if err != nil {
		t.Fatalf("get bin requests: %v", err)
	}
	if len(requests) != 0 {
		t.Errorf("request count = %d, want 0", len(requests))
	}
	if requests == nil {
		t.Error("requests = nil, want empty slice")
	}
}
