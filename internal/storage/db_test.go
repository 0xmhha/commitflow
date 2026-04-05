package storage

import (
	"database/sql"
	"testing"
)

// testDB creates an in-memory SQLite database with migrations applied.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := applyPragmas(db); err != nil {
		db.Close()
		t.Fatalf("apply pragmas: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenDB_InMemory(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	defer db.Close()

	if err := applyPragmas(db); err != nil {
		t.Fatalf("apply pragmas: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestRunMigrations_CreatesTablesAndIndices(t *testing.T) {
	db := testDB(t)

	// Verify all expected tables exist.
	tables := []string{"commit_analyses", "upstream_configs", "upstream_syncs", "ai_call_log", "schema_version"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}

	// Verify indices.
	indices := []string{
		"idx_commit_analyses_hash",
		"idx_commit_analyses_category",
		"idx_upstream_syncs_commit",
		"idx_upstream_syncs_status",
	}
	for _, idx := range indices {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := testDB(t)

	// Running migrations again should not fail.
	if err := RunMigrations(db); err != nil {
		t.Fatalf("second migration run failed: %v", err)
	}

	// Schema version should still be 1.
	var version int
	err := db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("query schema version: %v", err)
	}
	if version != 1 {
		t.Errorf("schema version = %d, want 1", version)
	}
}
