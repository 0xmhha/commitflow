package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	_ "modernc.org/sqlite"
)

// OpenDB opens a SQLite database at the given path with optimized pragmas.
func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if err := applyPragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	// Enforce restrictive file permissions (owner-only read/write).
	enforceFilePermissions(path)

	return db, nil
}

// enforceFilePermissions restricts the database file and its WAL/SHM
// sidecar files to owner-only read/write (0600).
func enforceFilePermissions(path string) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(p)
		if err != nil {
			continue // file may not exist yet
		}
		if info.Mode().Perm()&0o077 != 0 {
			if err := os.Chmod(p, 0o600); err != nil {
				slog.Debug("could not enforce db file permissions", "path", p, "error", err)
			}
		}
	}
}

// CloseDB closes the given database connection.
func CloseDB(db *sql.DB) error {
	if err := db.Close(); err != nil {
		return fmt.Errorf("close sqlite db: %w", err)
	}
	return nil
}

func applyPragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA cache_size=-64000",
		"PRAGMA mmap_size=268435456",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("apply pragma %q: %w", pragma, err)
		}
	}

	return nil
}
