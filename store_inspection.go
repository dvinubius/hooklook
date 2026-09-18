package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

var ErrRequestNotFound = errors.New("request not found")

type BinAccess struct {
	Bin            Bin    `json:"bin"`
	Owner          bool   `json:"owner"`
	SharingEnabled bool   `json:"sharingEnabled"`
	InviteID       string `json:"inviteId,omitempty"`
}

func (s *Store) ownedBin(secret string) (Bin, error) {
	if secret == "" {
		return Bin{}, ErrBinNotFound
	}
	var bin Bin
	var created string
	var expires int64
	err := s.db.QueryRow(`SELECT code, created_at, expires_at, total_body_bytes FROM bins WHERE owner_digest = ? AND expires_at > ?`, digestSecret(secret), time.Now().Unix()).Scan(&bin.Code, &created, &expires, &bin.TotalBodyBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return Bin{}, ErrBinNotFound
	}
	if err != nil {
		return Bin{}, err
	}
	bin.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return Bin{}, err
	}
	bin.ExpiresAt = time.Unix(expires, 0).UTC()
	return bin, nil
}

func (s *Store) access(code, secret, invite string) (BinAccess, error) {
	var access BinAccess
	var created, ownerDigest, storedInvite string
	var expires int64
	err := s.db.QueryRow(`SELECT code, created_at, expires_at, total_body_bytes, owner_digest, invite_id, sharing_enabled FROM bins WHERE code = ? AND expires_at > ?`, code, time.Now().Unix()).Scan(&access.Bin.Code, &created, &expires, &access.Bin.TotalBodyBytes, &ownerDigest, &storedInvite, &access.SharingEnabled)
	if errors.Is(err, sql.ErrNoRows) {
		return access, ErrBinNotFound
	}
	if err != nil {
		return access, err
	}
	access.Owner = secret != "" && ownerDigest != "" && subtle.ConstantTimeCompare([]byte(digestSecret(secret)), []byte(ownerDigest)) == 1
	guest := invite != "" && storedInvite != "" && access.SharingEnabled && subtle.ConstantTimeCompare([]byte(invite), []byte(storedInvite)) == 1
	if !access.Owner && !guest {
		return BinAccess{}, ErrBinNotFound
	}
	access.Bin.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return BinAccess{}, err
	}
	access.Bin.ExpiresAt = time.Unix(expires, 0).UTC()
	if access.Owner {
		access.InviteID = storedInvite
	}
	return access, nil
}

type RequestDetail struct {
	SummarizedRequest
	Headers HeaderMap `json:"headers"`
	RawBody []byte    `json:"rawBody"`
}

func (s *Store) requestDetail(code, id string) (RequestDetail, error) {
	requestID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || requestID <= 0 {
		return RequestDetail{}, ErrRequestNotFound
	}
	var detail RequestDetail
	var received string
	var headers []byte
	err = s.db.QueryRow(`SELECT id, method, path, raw_query, created_at, content_type, body_size_kib, headers_json, raw_body FROM requests WHERE bin_code = ? AND id = ?`, code, requestID).Scan(&requestID, &detail.Method, &detail.Path, &detail.RawQuery, &received, &detail.ContentType, &detail.BodySizeKiB, &headers, &detail.RawBody)
	if errors.Is(err, sql.ErrNoRows) {
		return detail, ErrRequestNotFound
	}
	if err != nil {
		return detail, err
	}
	detail.Id = strconv.FormatInt(requestID, 10)
	detail.ReceivedAt, err = time.Parse(time.RFC3339Nano, received)
	if err != nil {
		return detail, err
	}
	if err := json.Unmarshal(headers, &detail.Headers); err != nil {
		return detail, err
	}
	detail.HeaderCount = len(detail.Headers)
	return detail, nil
}

func (s *Store) deleteRequest(code, id string) error {
	requestID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || requestID <= 0 {
		return ErrRequestNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var size int64
	err = tx.QueryRow(`SELECT length(raw_body) FROM requests WHERE bin_code = ? AND id = ?`, code, requestID).Scan(&size)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRequestNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM requests WHERE bin_code = ? AND id = ?`, code, requestID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE bins SET total_body_bytes = total_body_bytes - ? WHERE code = ?`, size, code); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) clearRequests(code string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM requests WHERE bin_code = ?`, code); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE bins SET total_body_bytes = 0 WHERE code = ?`, code); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) setSharing(code string, enabled bool) error {
	result, err := s.db.Exec(`UPDATE bins SET sharing_enabled = ? WHERE code = ?`, enabled, code)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrBinNotFound
	}
	return nil
}
