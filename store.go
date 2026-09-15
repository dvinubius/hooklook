package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	storeCodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	binCodeLength     = 22
	requestCodeLength = 8
	defaultBinTTL     = 7 * 24 * time.Hour
)

var (
	ErrBinNotFound = errors.New("bin not found")
)

type Bin struct {
	Code      string                   `json:"code"`
	CreatedAt time.Time                `json:"createdAt"`
	ExpiresAt time.Time                `json:"expiresAt"`
	Requests  map[string]ParsedRequest `json:"requests"`
}

type Store struct {
	Bins         map[string]Bin // bin code to bin
	mu           sync.RWMutex
	generateCode func(int) (string, error)
}

func newBinStore() *Store {
	return &Store{
		Bins:         make(map[string]Bin),
		generateCode: generateCode,
	}
}

func generateCode(length int) (string, error) {
	return randomString(storeCodeAlphabet, length)
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

func (s *Store) createBin() (Bin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	code := ""
	for code == "" {
		candidate, err := s.generateCode(binCodeLength)
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
		Requests:  make(map[string]ParsedRequest),
	}
	bin.ExpiresAt = bin.CreatedAt.Add(defaultBinTTL)
	s.Bins[code] = bin

	return bin, nil
}

// TODO see if any error can even occur. if not, remove from signature
func (s *Store) getAllBins() ([]Bin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bins := []Bin{}
	for _, v := range s.Bins {
		bins = append(bins, v)
	}

	return bins, nil
}

func (s *Store) hasBin(binCode string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.Bins[binCode]
	return ok
}

func (s *Store) saveRequest(parsedReq ParsedRequest, binCode string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.Bins[binCode]
	if !ok {
		return "", ErrBinNotFound
	}

	requestCode := ""
	for requestCode == "" {
		candidate, err := s.generateCode(requestCodeLength)
		if err != nil {
			return "", fmt.Errorf("generate request code: %w", err)
		}
		_, ok := s.Bins[binCode].Requests[candidate]
		if !ok {
			requestCode = candidate
		}
	}

	parsedReq.Id = requestCode
	s.Bins[binCode].Requests[requestCode] = parsedReq

	return requestCode, nil
}

func (s *Store) getBinRequests(binCode string) ([]SummarizedRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.Bins[binCode]
	if !ok {
		return []SummarizedRequest{}, ErrBinNotFound
	}

	requests := []SummarizedRequest{}
	for _, v := range s.Bins[binCode].Requests {
		summarizedRequest := SummarizedRequest{
			Id:          v.Id,
			Method:      v.Method,
			Path:        v.Path,
			ReceiptTime: v.ReceiptTime,
		}
		requests = append(requests, summarizedRequest)
	}

	return requests, nil
}
