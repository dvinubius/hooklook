package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/mattn/go-sqlite3"
)

const defaultMaxStore int64 = 5_000_000_000

// StoreCapacity describes SQLite's main-file ceiling. Reusable pages are
// already in the file, so they count toward room for a new write even though
// databaseBytes does not shrink when rows are deleted.
type StoreCapacity struct {
	MaxBytes       int64 `json:"maxBytes"`
	DatabaseBytes  int64 `json:"databaseBytes"`
	ReusableBytes  int64 `json:"reusableBytes"`
	AvailableBytes int64 `json:"availableBytes"`
	Full           bool  `json:"full"`
	availablePages int64
}

func maxStoreFromEnvironment() (int64, error) {
	value := os.Getenv("MAX_STORE")
	if value == "" {
		return defaultMaxStore, nil
	}
	bytes, err := strconv.ParseInt(value, 10, 64)
	if err != nil || bytes <= 0 {
		return 0, fmt.Errorf("MAX_STORE must be a positive number of bytes")
	}
	return bytes, nil
}

// SQLite's page cap applies to the main database file. A separate filesystem
// reserve is needed for journals, backups and other maintenance files.
func (s *Store) configureCapacity(maxBytes int64) error {
	if err := s.db.QueryRow(`PRAGMA page_size`).Scan(&s.pageSize); err != nil {
		return err
	}
	if maxBytes < s.pageSize {
		return fmt.Errorf("MAX_STORE is smaller than one SQLite page")
	}
	s.maxStorePages = maxBytes / s.pageSize
	var actual int64
	if err := s.db.QueryRow(fmt.Sprintf(`PRAGMA max_page_count = %d`, s.maxStorePages)).Scan(&actual); err != nil {
		return err
	}
	if actual < s.maxStorePages {
		return fmt.Errorf("SQLite max_page_count is %d, below requested %d", actual, s.maxStorePages)
	}
	// SQLite cannot lower max_page_count beneath the file's current page count.
	// Keep the requested cap in Store: storeCapacity marks the database full and
	// the write precheck rejects new bins/captures while existing data remains
	// readable and removable. Reassert the cap on each check as pages shrink.
	return nil
}

func (s *Store) storeCapacity() (StoreCapacity, error) {
	if s.maxStorePages == 0 {
		return StoreCapacity{}, fmt.Errorf("SQLite capacity is not configured")
	}
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		return StoreCapacity{}, err
	}
	defer conn.Close()
	// max_page_count belongs to a SQLite connection, so reassert it before
	// both reporting capacity and growing writes if database/sql replaced it.
	var ceiling int64
	if err := conn.QueryRowContext(context.Background(), fmt.Sprintf(`PRAGMA max_page_count = %d`, s.maxStorePages)).Scan(&ceiling); err != nil {
		return StoreCapacity{}, err
	}
	var pages, free int64
	if err := conn.QueryRowContext(context.Background(), `PRAGMA page_count`).Scan(&pages); err != nil {
		return StoreCapacity{}, err
	}
	if err := conn.QueryRowContext(context.Background(), `PRAGMA freelist_count`).Scan(&free); err != nil {
		return StoreCapacity{}, err
	}
	available := max(0, s.maxStorePages-pages+free)
	capacity := StoreCapacity{
		MaxBytes:       s.maxStorePages * s.pageSize,
		DatabaseBytes:  pages * s.pageSize,
		ReusableBytes:  free * s.pageSize,
		AvailableBytes: available * s.pageSize,
		Full:           ceiling != s.maxStorePages || available < 2,
		availablePages: available,
	}
	return capacity, nil
}

// checkCapacity rejects a write before starting it when the pages it is
// likely to need cannot fit. SQLite's own page cap remains the final guard.
func (s *Store) checkCapacity(estimatedBytes int64) error {
	if s.maxStorePages == 0 {
		return nil
	} // permits the hard-cap test to bypass the advisory check
	capacity, err := s.storeCapacity()
	if err != nil {
		return err
	}
	needed := (estimatedBytes+s.pageSize-1)/s.pageSize + 1
	if capacity.Full || capacity.availablePages < needed {
		return ErrStoreFull
	}
	return nil
}

func classifyStoreError(err error) error {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code == sqlite3.ErrFull {
		return fmt.Errorf("%w: %v", ErrStoreFull, err)
	}
	return err
}

func configureStore(db *sql.DB) (*Store, error) {
	maximum, err := maxStoreFromEnvironment()
	if err != nil {
		return nil, err
	}
	s := newBinStore(db)
	if err := s.configureCapacity(maximum); err != nil {
		return nil, err
	}
	return s, nil
}
