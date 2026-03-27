package storage

import (
	"database/sql"
	"fmt"
)

type migration struct {
	version int
	up      string
}

var migrations = []migration{
	{
		version: 1,
		up:      migration1,
	},
}

const migration1 = `
CREATE TABLE IF NOT EXISTS commit_analyses (
    id INTEGER PRIMARY KEY,
    repo_path TEXT NOT NULL,
    commit_hash TEXT NOT NULL,
    parent_hash TEXT,
    author TEXT,
    author_email TEXT,
    commit_date TEXT,
    message TEXT,
    files_changed TEXT,
    diff_stats TEXT,
    category TEXT,
    summary TEXT,
    detailed_analysis TEXT,
    impact_score INTEGER,
    breaking_changes BOOLEAN DEFAULT FALSE,
    packages_affected TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_path, commit_hash)
);

CREATE TABLE IF NOT EXISTS upstream_configs (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    upstream_url TEXT NOT NULL,
    fork_path TEXT NOT NULL,
    last_synced_hash TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS upstream_syncs (
    id INTEGER PRIMARY KEY,
    config_id INTEGER REFERENCES upstream_configs(id),
    upstream_commit TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    applicability_reason TEXT,
    relevance_score INTEGER,
    applied_at TEXT,
    applied_commit TEXT,
    skip_reason TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(config_id, upstream_commit)
);

CREATE TABLE IF NOT EXISTS ai_call_log (
    id INTEGER PRIMARY KEY,
    call_type TEXT NOT NULL,
    commit_hash TEXT,
    model TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cost_usd REAL,
    duration_ms INTEGER,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_commit_analyses_hash ON commit_analyses(repo_path, commit_hash);
CREATE INDEX IF NOT EXISTS idx_commit_analyses_category ON commit_analyses(repo_path, category);
CREATE INDEX IF NOT EXISTS idx_upstream_syncs_commit ON upstream_syncs(config_id, upstream_commit);
CREATE INDEX IF NOT EXISTS idx_upstream_syncs_status ON upstream_syncs(config_id, status);
`

// RunMigrations applies all pending schema migrations to the database.
func RunMigrations(db *sql.DB) error {
	if err := ensureSchemaVersionTable(db); err != nil {
		return err
	}

	currentVersion, err := currentSchemaVersion(db)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return err
		}
	}

	return nil
}

func ensureSchemaVersionTable(db *sql.DB) error {
	const query = `
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL,
    applied_at TEXT DEFAULT CURRENT_TIMESTAMP
)`
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("ensure schema_version table: %w", err)
	}
	return nil
}

func currentSchemaVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("query schema version: %w", err)
	}
	return version, nil
}

func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", m.version, err)
	}

	if _, err := tx.Exec(m.up); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("run migration %d: %w", m.version, err)
	}

	if _, err := tx.Exec("INSERT INTO schema_version(version) VALUES(?)", m.version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %d: %w", m.version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", m.version, err)
	}

	return nil
}
