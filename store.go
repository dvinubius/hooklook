package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mattn/go-sqlite3"
)

const (
	storeCodeAlphabet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	defaultBinTTL      = 7 * 24 * time.Hour
	maxStoredRequests  = 500
	maxStoredBodyBytes = 100_000_000
)

var (
	ErrBinNotFound = errors.New("bin not found")
	ErrBinFull     = errors.New("bin storage limit reached")
)

type Bin struct {
	Code           string    `json:"code"`
	CreatedAt      time.Time `json:"createdAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
	TotalBodyBytes int64     `json:"totalBodyBytes"`
}

type BinSummary struct {
	Bin
	RequestCount int `json:"requestCount"`
}

type Store struct {
	db           *sql.DB
	generateCode func() (string, error)
}

func newBinStore(db *sql.DB) *Store {
	return &Store{
		db:           db,
		generateCode: generateCode,
	}
}

// Bin code words. 36 × 36 × 100 two-digit suffixes gives 129,600 codes;
// collisions are retried against the database.
var (
	codeAdjectives = [...]string{
		"amber", "bold", "bright", "brisk", "calm", "cheery", "clear", "cozy", "crisp",
		"dapper", "eager", "fancy", "fleet", "fresh", "frosty", "gentle", "golden", "green",
		"happy", "jolly", "keen", "lively", "lucky", "mellow", "merry", "misty", "nimble",
		"plucky", "quiet", "rosy", "silver", "sunny", "swift", "tidy", "witty", "zesty",
	}
	codeNouns = [...]string{
		"acorn", "aspen", "badger", "beacon", "birch", "canyon", "cedar", "clover", "comet",
		"coral", "cricket", "dolphin", "ember", "falcon", "fern", "glacier", "harbor", "heron",
		"island", "lantern", "maple", "marten", "meadow", "orchard", "otter", "pebble", "puffin",
		"raven", "river", "sparrow", "summit", "thistle", "tundra", "walrus", "willow", "zephyr",
	}
)

func generateCode() (string, error) {
	a, err := randomIndex(len(codeAdjectives))
	if err != nil {
		return "", err
	}
	n, err := randomIndex(len(codeNouns))
	if err != nil {
		return "", err
	}
	number, err := randomIndex(100)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%02d", codeAdjectives[a], codeNouns[n], number), nil
}

func randomString(alphabet string, length int) (string, error) {
	out := make([]byte, length)
	for i := range out {
		index, err := randomIndex(len(alphabet))
		if err != nil {
			return "", err
		}
		out[i] = alphabet[index]
	}
	return string(out), nil
}

// randomIndex returns a uniform index in [0, n) for n <= 256, rejecting bytes
// past the last whole multiple of n so no index is favoured.
func randomIndex(n int) (int, error) {
	validByteLimit := 256 - (256 % n)
	for {
		var randomByte [1]byte
		if _, err := rand.Read(randomByte[:]); err != nil {
			return 0, fmt.Errorf("read cryptographic randomness: %w", err)
		}
		if int(randomByte[0]) < validByteLimit {
			return int(randomByte[0]) % n, nil
		}
	}
}

func isUniqueConstraint(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.As(err, &sqliteErr) &&
		(sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
			sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey)
}

// PUBLIC CRUD

func (s *Store) createBin() (Bin, error) {
	bin, _, err := s.createOwnedBin()
	return bin, err
}

func (s *Store) createOwnedBin() (Bin, string, error) {
	for {
		code, err := s.generateCode()
		if err != nil {
			return Bin{}, "", fmt.Errorf("generate bin code: %w", err)
		}

		bin := Bin{
			Code:      code,
			CreatedAt: time.Now().UTC().Truncate(time.Second),
		}
		bin.ExpiresAt = bin.CreatedAt.Add(defaultBinTTL)
		owner, err := randomString(storeCodeAlphabet, 48)
		if err != nil {
			return Bin{}, "", err
		}
		invite, err := randomString(storeCodeAlphabet, 48)
		if err != nil {
			return Bin{}, "", err
		}
		_, err = s.db.Exec(`
            INSERT INTO bins (code, created_at, expires_at, total_body_bytes, owner_digest, invite_id)
            VALUES (?, ?, ?, ?, ?, ?)
        `, bin.Code, bin.CreatedAt.Format(time.RFC3339Nano),
			bin.ExpiresAt.Unix(), bin.TotalBodyBytes, digestSecret(owner), invite)
		if err != nil {
			if isUniqueConstraint(err) {
				// The database is the authority on uniqueness. Generate a new code
				// and retry if another bin already has this one. Expected retries are
				// intentionally excluded from the minimal DB metrics.
				continue
			}
			return Bin{}, "", err
		}

		return bin, owner, nil
	}
}

func (s *Store) saveRequest(parsedReq ParsedRequest, binCode string) (string, error) {
	headersJSON, err := json.Marshal(parsedReq.Headers)
	if err != nil {
		return "", fmt.Errorf("encode request headers: %w", err)
	}
	receivedAt := parsedReq.ReceiptTime.Format(time.RFC3339Nano)
	bodyBytes := int64(len(parsedReq.RawBody))

	tx, err := s.db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin save request: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE bins
		SET total_body_bytes = total_body_bytes + ?
		WHERE code = ?
			AND expires_at > ?
			AND total_body_bytes + ? <= ?
			AND (SELECT COUNT(*) FROM requests WHERE bin_code = bins.code) < ?
	`, bodyBytes, binCode, time.Now().UTC().Unix(),
		bodyBytes, maxStoredBodyBytes, maxStoredRequests)
	if err != nil {
		return "", fmt.Errorf("update total body bytes: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("read bin update result: %w", err)
	}
	if updated == 0 {
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM bins WHERE code = ? AND expires_at > ?)`, binCode, time.Now().UTC().Unix()).Scan(&exists); err != nil {
			return "", fmt.Errorf("check bin availability: %w", err)
		}
		if !exists {
			return "", ErrBinNotFound
		}
		return "", ErrBinFull
	}

	result, err = tx.Exec(`
		INSERT INTO requests (
			bin_code, created_at, method, path, raw_query, headers_json,
			content_type, raw_body, body_size_kib
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, binCode, receivedAt, parsedReq.Method,
		parsedReq.Path, parsedReq.RawQuery, headersJSON, parsedReq.ContentType,
		parsedReq.RawBody, parsedReq.BodySizeKiB)
	if err != nil {
		return "", fmt.Errorf("insert request: %w", err)
	}
	requestID, err := result.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("read inserted request ID: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit save request: %w", err)
	}

	return strconv.FormatInt(requestID, 10), nil
}

func (s *Store) getAllBins() ([]BinSummary, error) {
	rows, err := s.db.Query(`
		SELECT bins.code, bins.created_at, bins.expires_at,
			bins.total_body_bytes, COUNT(requests.id)
		FROM bins
		LEFT JOIN requests ON requests.bin_code = bins.code
		GROUP BY bins.code
		ORDER BY bins.created_at DESC, bins.code
	`)
	if err != nil {
		return nil, fmt.Errorf("list bins: %w", err)
	}
	defer rows.Close()

	bins := []BinSummary{}
	for rows.Next() {
		var bin BinSummary
		var createdAt string
		var expiresAt int64
		if err := rows.Scan(&bin.Code, &createdAt, &expiresAt,
			&bin.TotalBodyBytes, &bin.RequestCount); err != nil {
			return nil, fmt.Errorf("scan bin: %w", err)
		}
		bin.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse bin creation time: %w", err)
		}
		bin.ExpiresAt = time.Unix(expiresAt, 0).UTC()
		bins = append(bins, bin)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bins: %w", err)
	}
	return bins, nil
}

func (s *Store) getBinRequests(binCode string) ([]SummarizedRequest, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM bins WHERE code = ?)`, binCode).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check bin: %w", err)
	}
	if !exists {
		return []SummarizedRequest{}, ErrBinNotFound
	}

	rows, err := s.db.Query(`
		SELECT id, method, path, raw_query, created_at, content_type,
			body_size_kib, headers_json
		FROM requests
		WHERE bin_code = ?
		ORDER BY id ASC
	`, binCode)
	if err != nil {
		return nil, fmt.Errorf("get bin requests: %w", err)
	}
	defer rows.Close()

	requests := []SummarizedRequest{}
	for rows.Next() {
		var request SummarizedRequest
		var requestID int64
		var receivedAt string
		var headersJSON []byte
		if err := rows.Scan(
			&requestID,
			&request.Method,
			&request.Path,
			&request.RawQuery,
			&receivedAt,
			&request.ContentType,
			&request.BodySizeKiB,
			&headersJSON,
		); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		var headers HeaderMap
		if err := json.Unmarshal(headersJSON, &headers); err != nil {
			return nil, fmt.Errorf("decode request headers: %w", err)
		}
		receiptTime, err := time.Parse(time.RFC3339Nano, receivedAt)
		if err != nil {
			return nil, fmt.Errorf("parse request created_at: %w", err)
		}
		request.Id = strconv.FormatInt(requestID, 10)
		request.ReceivedAt = receiptTime
		request.HeaderCount = len(headers)
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate requests: %w", err)
	}

	return requests, nil
}

func digestSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
