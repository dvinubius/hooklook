package main

import (
	"fmt"
	"os"
)

// VACUUM INTO is SQLite's consistent online backup operation. The destination
// must not exist, so an interrupted backup cannot silently be mistaken for a
// fresh successful one on retry.
func backupDatabase(source, destination string) error {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return fmt.Errorf("backup destination already exists")
	}
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(destination)
		return err
	}
	db, err := openDB(source)
	if err != nil {
		_ = os.Remove(destination)
		return err
	}
	defer db.Close()
	if _, err := db.Exec(`VACUUM INTO ?`, destination); err != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("online SQLite backup: %w", err)
	}
	return nil
}
