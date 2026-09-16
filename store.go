package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mattn/go-sqlite3"
)

const (
	storeCodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	binCodeLength     = 22
	defaultBinTTL     = 7 * 24 * time.Hour
	maxStoredRequests = 500
	maxStoredBodyKiB  = 10 * 1024

	// Token secrets are copied by hand from an email, so the alphabet is
	// lowercase-only, drops the 0/o and 1/l lookalikes, and contains no
	// characters that break double-click selection. 32 chars × 12 ≈ 60 bits,
	// ample for a use-bounded, revocable token (ADR 0002).
	creationTokenAlphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	creationTokenLength   = 18
)

var (
	ErrBinNotFound           = errors.New("bin not found")
	ErrBinFull               = errors.New("bin storage limit reached")
	ErrCreationTokenInvalid  = errors.New("creation token invalid")
	ErrCreationTokenNotFound = errors.New("creation token not found")
)

type Bin struct {
	Code               string    `json:"code"`
	CreatedAt          time.Time `json:"createdAt"`
	ExpiresAt          time.Time `json:"expiresAt"`
	TotalStoredBodyKiB int       `json:"totalStoredBodyKiB"`
}

type BinSummary struct {
	Bin
	RequestCount int `json:"requestCount"`
}

type CreationToken struct {
	ID        string     `json:"id"`
	Label     string     `json:"label"`
	CreatedAt time.Time  `json:"createdAt"`
	MaxUses   int        `json:"maxUses"`
	UseCount  int        `json:"useCount"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

type Store struct {
	db                    *sql.DB
	generateCode          func() (string, error)
	generateCreationToken func() (id, token string, err error)
}

func newBinStore(db *sql.DB) *Store {
	return &Store{
		db:                    db,
		generateCode:          generateCode,
		generateCreationToken: generateCreationToken,
	}
}

func generateCode() (string, error) {
	return randomString(storeCodeAlphabet, binCodeLength)
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

func generateCreationToken() (string, string, error) {
	idBytes := make([]byte, 9)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", fmt.Errorf("generate token ID: %w", err)
	}
	secret, err := randomString(creationTokenAlphabet, creationTokenLength)
	if err != nil {
		return "", "", fmt.Errorf("generate token secret: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(idBytes), "hklk_" + secret, nil
}

func (s *Store) issueCreationToken(label string, maxUses int) (CreationToken, string, error) {
	for {
		id, token, err := s.generateCreationToken()
		if err != nil {
			return CreationToken{}, "", err
		}

		now := time.Now().UTC().Truncate(time.Second)
		creationToken := CreationToken{
			ID:        id,
			Label:     label,
			CreatedAt: now,
			MaxUses:   maxUses,
		}
		hash := sha256.Sum256([]byte(token))
		_, err = s.db.Exec(`
			INSERT INTO creation_tokens
				(id, token_hash, label, created_at, max_uses, use_count)
			VALUES (?, ?, ?, ?, ?, 0)
		`, creationToken.ID, hash[:], creationToken.Label, creationToken.CreatedAt.Unix(),
			creationToken.MaxUses)
		if err != nil {
			if isUniqueConstraint(err) {
				continue
			}
			return CreationToken{}, "", fmt.Errorf("insert creation token: %w", err)
		}

		return creationToken, token, nil
	}
}

// consumeCreationToken spends one use of a token and reports how many uses the
// token has left afterwards. The remaining count is surfaced to the frontend so
// it can warn before a token runs out; it comes straight from the UPDATE via
// RETURNING, so it is consistent with the use it just recorded.
func (s *Store) consumeCreationToken(token string) (int, error) {
	hash := sha256.Sum256([]byte(token))
	var usesLeft int
	err := s.db.QueryRow(`
		UPDATE creation_tokens
		SET use_count = use_count + 1
		WHERE token_hash = ?
			AND revoked_at IS NULL
			AND use_count < max_uses
		RETURNING max_uses - use_count
	`, hash[:]).Scan(&usesLeft)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrCreationTokenInvalid
	}
	if err != nil {
		return 0, fmt.Errorf("consume creation token: %w", err)
	}

	return usesLeft, nil
}

func (s *Store) revokeCreationToken(id string) error {
	result, err := s.db.Exec(`
		UPDATE creation_tokens
		SET revoked_at = ?
		WHERE id = ? AND revoked_at IS NULL
	`, time.Now().UTC().Unix(), id)
	if err != nil {
		return fmt.Errorf("revoke creation token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check token revocation result: %w", err)
	}
	if rowsAffected == 0 {
		return ErrCreationTokenNotFound
	}
	return nil
}

func (s *Store) listCreationTokens() ([]CreationToken, error) {
	rows, err := s.db.Query(`
		SELECT id, label, created_at, max_uses, use_count, revoked_at
		FROM creation_tokens
		ORDER BY created_at DESC, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list creation tokens: %w", err)
	}
	defer rows.Close()

	tokens := []CreationToken{}
	for rows.Next() {
		var token CreationToken
		var createdAt int64
		var revokedAt sql.NullInt64
		if err := rows.Scan(
			&token.ID,
			&token.Label,
			&createdAt,
			&token.MaxUses,
			&token.UseCount,
			&revokedAt,
		); err != nil {
			return nil, fmt.Errorf("scan creation token: %w", err)
		}

		token.CreatedAt = time.Unix(createdAt, 0).UTC()
		if revokedAt.Valid {
			value := time.Unix(revokedAt.Int64, 0).UTC()
			token.RevokedAt = &value
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate creation tokens: %w", err)
	}

	return tokens, nil
}

func isUniqueConstraint(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.As(err, &sqliteErr) &&
		(sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
			sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey)
}

// ADMIN CRUD

func (s *Store) getAllBins() ([]BinSummary, error) {
	rows, err := s.db.Query(`
		SELECT bins.code, bins.created_at, bins.expires_at,
			bins.total_stored_body_kib, COUNT(requests.id)
		FROM bins
		LEFT JOIN requests ON requests.bin_code = bins.code
		GROUP BY bins.code
	`)
	if err != nil {
		return nil, fmt.Errorf("get all bins: %v", err)
	}
	defer rows.Close()

	bins := []BinSummary{}

	for rows.Next() {
		var bin BinSummary
		var createdAt string
		var expiresAt int64
		var totalStoredBodyKib int
		if err := rows.Scan(
			&bin.Code,
			&createdAt,
			&expiresAt,
			&totalStoredBodyKib,
			&bin.RequestCount,
		); err != nil {
			return nil, fmt.Errorf("scan bin: %w", err)
		}

		parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return bins, fmt.Errorf("parse created_at: %w", err)
		}
		bin.Bin.CreatedAt = parsedCreatedAt
		bin.Bin.ExpiresAt = time.Unix(expiresAt, 0).UTC()
		bin.Bin.TotalStoredBodyKiB = totalStoredBodyKib

		bins = append(bins, bin)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bins: %w", err)
	}

	return bins, nil
}

func (s *Store) deleteBin(code string) error {
	result, err := s.db.Exec(`
		DELETE FROM bins
		WHERE code = ?
	`, code)
	if err != nil {
		return fmt.Errorf("delete bin: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result: %w", err)
	}
	if rowsAffected == 0 {
		return ErrBinNotFound
	}
	return nil
}

// PUBLIC CRUD

func (s *Store) createBin() (Bin, error) {
	for {
		code, err := s.generateCode()
		if err != nil {
			return Bin{}, fmt.Errorf("generate bin code: %w", err)
		}

		bin := Bin{
			Code:      code,
			CreatedAt: time.Now().UTC().Truncate(time.Second),
		}
		bin.ExpiresAt = bin.CreatedAt.Add(defaultBinTTL)
		_, err = s.db.Exec(`
			INSERT INTO bins (code, created_at, expires_at, total_stored_body_kib)
			VALUES (?, ?, ?, ?)
		`, bin.Code, bin.CreatedAt.Format(time.RFC3339Nano),
			bin.ExpiresAt.Unix(), bin.TotalStoredBodyKiB)
		if err != nil {
			if isUniqueConstraint(err) {
				// The database is the authority on uniqueness. Generate a new code
				// and retry if another bin already has this one. Expected retries are
				// intentionally excluded from the minimal DB metrics.
				continue
			}
			return Bin{}, err
		}

		return bin, nil
	}
}

func (s *Store) saveRequest(parsedReq ParsedRequest, binCode string) (string, error) {
	headersJSON, err := json.Marshal(parsedReq.Headers)
	if err != nil {
		return "", fmt.Errorf("encode request headers: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin save request: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE bins
		SET total_stored_body_kib = total_stored_body_kib + ?
		WHERE code = ?
			AND expires_at > ?
			AND total_stored_body_kib + ? <= ?
			AND (SELECT COUNT(*) FROM requests WHERE bin_code = bins.code) < ?
	`, parsedReq.BodySizeKiB, binCode, time.Now().UTC().Unix(),
		parsedReq.BodySizeKiB, maxStoredBodyKiB, maxStoredRequests)
	if err != nil {
		return "", fmt.Errorf("update total stored body (KiB): %w", err)
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
	`, binCode, parsedReq.ReceiptTime.Format(time.RFC3339Nano), parsedReq.Method,
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

func (s *Store) getBinRequests(binCode string) ([]SummarizedRequest, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM bins WHERE code = ?)`, binCode).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check bin: %w", err)
	}
	if !exists {
		return []SummarizedRequest{}, ErrBinNotFound
	}

	rows, err := s.db.Query(`
		SELECT id, method, path, created_at
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
		if err := rows.Scan(&requestID, &request.Method, &request.Path, &receivedAt); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		receiptTime, err := time.Parse(time.RFC3339Nano, receivedAt)
		if err != nil {
			return nil, fmt.Errorf("parse request created_at: %w", err)
		}
		request.Id = strconv.FormatInt(requestID, 10)
		request.ReceiptTime = receiptTime
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate requests: %w", err)
	}

	return requests, nil
}
