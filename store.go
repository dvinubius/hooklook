package main

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

const (
	binCodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	binCodeLength   = 22
	defaultBinTTL   = 7 * 24 * time.Hour
)

type Bin struct {
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Store struct {
	Bins         map[string]Bin // bin code to bin
	mu           sync.RWMutex
	generateCode func() (string, error)
}

func newBinStore() *Store {
	return &Store{
		Bins:         make(map[string]Bin),
		generateCode: generateBinCode,
	}
}

func generateBinCode() (string, error) {
	return randomString(binCodeAlphabet, binCodeLength)
}

func randomString(alphabet string, length int) (string, error) {
	out := make([]byte, length)
	validByteLimit := 256 - (256 % len(alphabet))

	for i := range out {
		for {
			var randomByte [1]byte
			if _, err := rand.Read(randomByte[:]); err != nil {
				return "", fmt.Errorf("read cryptographic randomness: %w", err)
			}
			if int(randomByte[0]) >= validByteLimit {
				continue
			}

			out[i] = alphabet[int(randomByte[0])%len(alphabet)]
			break
		}
	}

	return string(out), nil
}

// CRUD

func (s *Store) create() (Bin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	code := ""
	for code == "" {
		candidate, err := s.generateCode()
		if err != nil {
			return Bin{}, fmt.Errorf("generate bin code: %w", err)
		}
		_, ok := s.Bins[candidate]
		if !ok {
			code = candidate
		}
	}

	var bin = Bin{
		Code:      code,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	bin.ExpiresAt = bin.CreatedAt.Add(defaultBinTTL)
	s.Bins[code] = bin

	return bin, nil
}
