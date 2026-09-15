package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGenerateShortCode(t *testing.T) {
	for range 100 {
		code, err := generateBinCode()
		if err != nil {
			t.Fatalf("generate bin code: %v", err)
		}
		if len(code) != binCodeLength {
			t.Fatalf("code length = %d, want %d", len(code), binCodeLength)
		}
		for _, character := range code {
			if !strings.ContainsRune(binCodeAlphabet, character) {
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
	store.generateCode = func() (string, error) {
		code := codes[0]
		codes = codes[1:]
		return code, nil
	}

	bin, err := store.create()
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
	store.generateCode = func() (string, error) {
		return "", wantErr
	}

	_, err := store.create()
	if !errors.Is(err, wantErr) {
		t.Fatalf("create bin error = %v, want wrapped %v", err, wantErr)
	}
}

func TestCreateStoresBinWithExpiry(t *testing.T) {
	store := newBinStore()

	bin, err := store.create()
	if err != nil {
		t.Fatalf("create bin: %v", err)
	}

	stored, ok := store.Bins[bin.Code]
	if !ok {
		t.Fatalf("bin %q was not stored", bin.Code)
	}
	if stored != bin {
		t.Errorf("stored bin = %#v, want %#v", stored, bin)
	}
	if got, want := bin.ExpiresAt.Sub(bin.CreatedAt), defaultBinTTL; got != want {
		t.Errorf("expiry duration = %s, want %s", got, want)
	}
	if bin.CreatedAt.Location() != time.UTC {
		t.Errorf("created at location = %s, want UTC", bin.CreatedAt.Location())
	}
}
