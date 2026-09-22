package main

import (
	"context"
	"fmt"
	"time"
)

const cleanupInterval = time.Minute

func (s *Store) deleteExpired(now time.Time) ([]string, error) {
	rows, err := s.db.Query(`DELETE FROM bins WHERE expires_at <= ? RETURNING code`, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("delete expired bins: %w", err)
	}
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return codes, nil
}

func cleanupWorker(ctx context.Context, s *Store, hub *EventHub, report func(error)) {
	clean := func() {
		streamAccessMu.Lock()
		defer streamAccessMu.Unlock()
		codes, err := s.deleteExpired(time.Now().UTC())
		if err != nil {
			report(err)
			return
		}
		for _, code := range codes {
			hub.closeBin(code)
		}
	}
	clean()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			clean()
		}
	}
}
