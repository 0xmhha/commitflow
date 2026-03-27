package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// UpstreamConfig holds the configuration for tracking a single upstream repository.
type UpstreamConfig struct {
	ID             int64
	Name           string
	UpstreamURL    string
	ForkPath       string
	LastSyncedHash string
	CreatedAt      string
}

// UpstreamSync represents the applicability tracking record for a single upstream commit.
type UpstreamSync struct {
	ID                  int64
	ConfigID            int64
	UpstreamCommit      string
	Status              string // pending|applicable|not_applicable|applied|skipped|conflict
	ApplicabilityReason string
	RelevanceScore      int
	AppliedAt           string
	AppliedCommit       string
	SkipReason          string
	CreatedAt           string
}

// SyncFilter specifies criteria for listing upstream syncs.
type SyncFilter struct {
	ConfigID int64
	Status   string
	Limit    int
	Offset   int
}

// CallLog records a single AI call for cost and token tracking.
type CallLog struct {
	CallType     string
	CommitHash   string
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	DurationMS   int
}

// SyncStore defines the persistence interface for upstream sync operations.
type SyncStore interface {
	// Config operations
	SaveConfig(ctx context.Context, cfg *UpstreamConfig) error
	GetConfig(ctx context.Context, name string) (*UpstreamConfig, error)
	GetConfigByID(ctx context.Context, id int64) (*UpstreamConfig, error)
	ListConfigs(ctx context.Context) ([]UpstreamConfig, error)
	UpdateLastSyncedHash(ctx context.Context, id int64, hash string) error

	// Sync operations
	SaveSync(ctx context.Context, sync *UpstreamSync) error
	GetSync(ctx context.Context, configID int64, commitHash string) (*UpstreamSync, error)
	ListSyncs(ctx context.Context, filter SyncFilter) ([]UpstreamSync, error)
	UpdateSyncStatus(ctx context.Context, id int64, status, reason string) error
	MarkApplied(ctx context.Context, id int64, appliedCommit string) error
	MarkSkipped(ctx context.Context, id int64, reason string) error
	CountByStatus(ctx context.Context, configID int64) (map[string]int, error)
	IsSynced(ctx context.Context, configID int64, commitHash string) (bool, error)

	// AI call log
	SaveCallLog(ctx context.Context, log *CallLog) error
}

type syncStore struct {
	db *sql.DB
}

// NewSyncStore returns a SyncStore backed by the given database.
func NewSyncStore(db *sql.DB) SyncStore {
	return &syncStore{db: db}
}

// SaveConfig inserts or updates an upstream config record.
func (s *syncStore) SaveConfig(ctx context.Context, cfg *UpstreamConfig) error {
	const query = `
INSERT INTO upstream_configs (name, upstream_url, fork_path, last_synced_hash)
VALUES (?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
    upstream_url     = excluded.upstream_url,
    fork_path        = excluded.fork_path,
    last_synced_hash = excluded.last_synced_hash`

	result, err := s.db.ExecContext(ctx, query,
		cfg.Name, cfg.UpstreamURL, cfg.ForkPath, cfg.LastSyncedHash,
	)
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if cfg.ID == 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("save config last insert id: %w", err)
		}
		cfg.ID = id
	}

	return nil
}

// GetConfig retrieves an upstream config by name.
func (s *syncStore) GetConfig(ctx context.Context, name string) (*UpstreamConfig, error) {
	const query = `
SELECT id, name, upstream_url, fork_path, COALESCE(last_synced_hash, ''), created_at
FROM upstream_configs
WHERE name = ?`

	row := s.db.QueryRowContext(ctx, query, name)
	cfg, err := scanUpstreamConfig(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get config %q: %w", name, err)
	}

	return cfg, nil
}

// GetConfigByID retrieves an upstream config by ID.
func (s *syncStore) GetConfigByID(ctx context.Context, id int64) (*UpstreamConfig, error) {
	const query = `
SELECT id, name, upstream_url, fork_path, COALESCE(last_synced_hash, ''), created_at
FROM upstream_configs
WHERE id = ?`

	row := s.db.QueryRowContext(ctx, query, id)
	cfg, err := scanUpstreamConfig(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get config by id %d: %w", id, err)
	}

	return cfg, nil
}

// ListConfigs returns all upstream configurations.
func (s *syncStore) ListConfigs(ctx context.Context) ([]UpstreamConfig, error) {
	const query = `
SELECT id, name, upstream_url, fork_path, COALESCE(last_synced_hash, ''), created_at
FROM upstream_configs
ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}
	defer rows.Close()

	var configs []UpstreamConfig
	for rows.Next() {
		cfg, err := scanUpstreamConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("list configs scan: %w", err)
		}
		configs = append(configs, *cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list configs rows: %w", err)
	}

	return configs, nil
}

// UpdateLastSyncedHash updates the last_synced_hash for a config record.
func (s *syncStore) UpdateLastSyncedHash(ctx context.Context, id int64, hash string) error {
	const query = `UPDATE upstream_configs SET last_synced_hash = ? WHERE id = ?`

	if _, err := s.db.ExecContext(ctx, query, hash, id); err != nil {
		return fmt.Errorf("update last synced hash: %w", err)
	}

	return nil
}

// SaveSync inserts or updates an upstream sync record.
func (s *syncStore) SaveSync(ctx context.Context, sync *UpstreamSync) error {
	const query = `
INSERT INTO upstream_syncs (
    config_id, upstream_commit, status, applicability_reason,
    relevance_score, applied_at, applied_commit, skip_reason
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(config_id, upstream_commit) DO UPDATE SET
    status                = excluded.status,
    applicability_reason  = excluded.applicability_reason,
    relevance_score       = excluded.relevance_score,
    applied_at            = excluded.applied_at,
    applied_commit        = excluded.applied_commit,
    skip_reason           = excluded.skip_reason`

	result, err := s.db.ExecContext(ctx, query,
		sync.ConfigID, sync.UpstreamCommit, sync.Status,
		sync.ApplicabilityReason, sync.RelevanceScore,
		nullableString(sync.AppliedAt), nullableString(sync.AppliedCommit),
		nullableString(sync.SkipReason),
	)
	if err != nil {
		return fmt.Errorf("save sync: %w", err)
	}

	if sync.ID == 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("save sync last insert id: %w", err)
		}
		sync.ID = id
	}

	return nil
}

// GetSync retrieves a sync record by config ID and upstream commit hash.
func (s *syncStore) GetSync(ctx context.Context, configID int64, commitHash string) (*UpstreamSync, error) {
	const query = `
SELECT id, config_id, upstream_commit, status,
       COALESCE(applicability_reason, ''), COALESCE(relevance_score, 0),
       COALESCE(applied_at, ''), COALESCE(applied_commit, ''),
       COALESCE(skip_reason, ''), created_at
FROM upstream_syncs
WHERE config_id = ? AND upstream_commit = ?`

	row := s.db.QueryRowContext(ctx, query, configID, commitHash)
	sync, err := scanUpstreamSync(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sync %s: %w", commitHash, err)
	}

	return sync, nil
}

// ListSyncs returns sync records matching the provided filter.
func (s *syncStore) ListSyncs(ctx context.Context, filter SyncFilter) ([]UpstreamSync, error) {
	query, args := buildSyncListQuery(filter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list syncs: %w", err)
	}
	defer rows.Close()

	var syncs []UpstreamSync
	for rows.Next() {
		sync, err := scanUpstreamSync(rows)
		if err != nil {
			return nil, fmt.Errorf("list syncs scan: %w", err)
		}
		syncs = append(syncs, *sync)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list syncs rows: %w", err)
	}

	return syncs, nil
}

// UpdateSyncStatus updates the status and applicability_reason for a sync record.
func (s *syncStore) UpdateSyncStatus(ctx context.Context, id int64, status, reason string) error {
	const query = `
UPDATE upstream_syncs
SET status = ?, applicability_reason = ?
WHERE id = ?`

	if _, err := s.db.ExecContext(ctx, query, status, reason, id); err != nil {
		return fmt.Errorf("update sync status %d: %w", id, err)
	}

	return nil
}

// MarkApplied records a successful cherry-pick for a sync record.
func (s *syncStore) MarkApplied(ctx context.Context, id int64, appliedCommit string) error {
	const query = `
UPDATE upstream_syncs
SET status = 'applied', applied_commit = ?, applied_at = CURRENT_TIMESTAMP
WHERE id = ?`

	if _, err := s.db.ExecContext(ctx, query, appliedCommit, id); err != nil {
		return fmt.Errorf("mark applied %d: %w", id, err)
	}

	return nil
}

// MarkSkipped records that a sync record was deliberately skipped.
func (s *syncStore) MarkSkipped(ctx context.Context, id int64, reason string) error {
	const query = `
UPDATE upstream_syncs
SET status = 'skipped', skip_reason = ?
WHERE id = ?`

	if _, err := s.db.ExecContext(ctx, query, reason, id); err != nil {
		return fmt.Errorf("mark skipped %d: %w", id, err)
	}

	return nil
}

// CountByStatus returns a map from status to count for a given config.
func (s *syncStore) CountByStatus(ctx context.Context, configID int64) (map[string]int, error) {
	const query = `
SELECT status, COUNT(*) AS cnt
FROM upstream_syncs
WHERE config_id = ?
GROUP BY status`

	rows, err := s.db.QueryContext(ctx, query, configID)
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("count by status scan: %w", err)
		}
		counts[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count by status rows: %w", err)
	}

	return counts, nil
}

// IsSynced returns true if the given commit has already been recorded for the config.
func (s *syncStore) IsSynced(ctx context.Context, configID int64, commitHash string) (bool, error) {
	const query = `
SELECT 1 FROM upstream_syncs
WHERE config_id = ? AND upstream_commit = ?
LIMIT 1`

	var exists int
	err := s.db.QueryRowContext(ctx, query, configID, commitHash).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("is synced %s: %w", commitHash, err)
	}

	return true, nil
}

// SaveCallLog inserts an AI call log record.
func (s *syncStore) SaveCallLog(ctx context.Context, log *CallLog) error {
	const query = `
INSERT INTO ai_call_log (call_type, commit_hash, model, input_tokens, output_tokens, cost_usd, duration_ms)
VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		log.CallType, log.CommitHash, log.Model,
		log.InputTokens, log.OutputTokens,
		log.CostUSD, log.DurationMS,
	)
	if err != nil {
		return fmt.Errorf("save call log: %w", err)
	}

	return nil
}

// scanUpstreamConfig scans a row into an UpstreamConfig.
func scanUpstreamConfig(s scanner) (*UpstreamConfig, error) {
	var cfg UpstreamConfig
	err := s.Scan(
		&cfg.ID, &cfg.Name, &cfg.UpstreamURL, &cfg.ForkPath,
		&cfg.LastSyncedHash, &cfg.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// scanUpstreamSync scans a row into an UpstreamSync.
func scanUpstreamSync(s scanner) (*UpstreamSync, error) {
	var sync UpstreamSync
	err := s.Scan(
		&sync.ID, &sync.ConfigID, &sync.UpstreamCommit, &sync.Status,
		&sync.ApplicabilityReason, &sync.RelevanceScore,
		&sync.AppliedAt, &sync.AppliedCommit,
		&sync.SkipReason, &sync.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &sync, nil
}

// buildSyncListQuery constructs a filtered query for listing upstream syncs.
func buildSyncListQuery(f SyncFilter) (string, []any) {
	query := `
SELECT id, config_id, upstream_commit, status,
       COALESCE(applicability_reason, ''), COALESCE(relevance_score, 0),
       COALESCE(applied_at, ''), COALESCE(applied_commit, ''),
       COALESCE(skip_reason, ''), created_at
FROM upstream_syncs
WHERE config_id = ?`

	args := []any{f.ConfigID}

	if f.Status != "" {
		query += " AND status = ?"
		args = append(args, f.Status)
	}

	query += " ORDER BY created_at ASC"

	if f.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, f.Limit)
	}

	if f.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, f.Offset)
	}

	return query, args
}

// nullableString returns nil when s is empty, otherwise returns &s.
// This prevents storing empty strings as NULL violations in optional columns.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
