package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/0xmhha/commitflow/internal/validate"
)

// DiffStats holds line-level change statistics for a commit.
type DiffStats struct {
	Additions  int `json:"additions"`
	Deletions  int `json:"deletions"`
	FilesCount int `json:"files_count"`
}

// CommitAnalysis represents a stored analysis result for a single commit.
type CommitAnalysis struct {
	ID               int64
	RepoPath         string
	CommitHash       string
	ParentHash       string
	Author           string
	AuthorEmail      string
	CommitDate       string
	Message          string
	FilesChanged     []string  // stored as JSON
	DiffStats        DiffStats // stored as JSON
	Category         string
	Summary          string
	DetailedAnalysis string
	ImpactScore      int
	BreakingChanges  bool
	PackagesAffected []string // stored as JSON
	CreatedAt        string
}

// AnalysisFilter specifies criteria for listing commit analyses.
type AnalysisFilter struct {
	RepoPath  string
	Category  string
	Since     string
	MinImpact int
	Limit     int
	Offset    int
}

// CommitStore defines the persistence interface for commit analyses.
type CommitStore interface {
	SaveAnalysis(ctx context.Context, analysis *CommitAnalysis) error
	GetAnalysis(ctx context.Context, repoPath, hash string) (*CommitAnalysis, error)
	ListAnalyses(ctx context.Context, filter AnalysisFilter) ([]CommitAnalysis, error)
	IsAnalyzed(ctx context.Context, repoPath, hash string) (bool, error)
	CountByCategory(ctx context.Context, repoPath string) (map[string]int, error)
}

type commitStore struct {
	db *sql.DB
}

// NewCommitStore returns a CommitStore backed by the given database.
func NewCommitStore(db *sql.DB) CommitStore {
	return &commitStore{db: db}
}

// SaveAnalysis inserts or replaces a commit analysis record.
func (s *commitStore) SaveAnalysis(ctx context.Context, analysis *CommitAnalysis) error {
	filesJSON, err := marshalJSON(analysis.FilesChanged)
	if err != nil {
		return fmt.Errorf("save analysis marshal files: %w", err)
	}

	diffJSON, err := marshalJSON(analysis.DiffStats)
	if err != nil {
		return fmt.Errorf("save analysis marshal diff_stats: %w", err)
	}

	pkgsJSON, err := marshalJSON(analysis.PackagesAffected)
	if err != nil {
		return fmt.Errorf("save analysis marshal packages: %w", err)
	}

	const query = `
INSERT INTO commit_analyses (
    repo_path, commit_hash, parent_hash, author, author_email,
    commit_date, message, files_changed, diff_stats, category,
    summary, detailed_analysis, impact_score, breaking_changes, packages_affected
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(repo_path, commit_hash) DO UPDATE SET
    parent_hash        = excluded.parent_hash,
    author             = excluded.author,
    author_email       = excluded.author_email,
    commit_date        = excluded.commit_date,
    message            = excluded.message,
    files_changed      = excluded.files_changed,
    diff_stats         = excluded.diff_stats,
    category           = excluded.category,
    summary            = excluded.summary,
    detailed_analysis  = excluded.detailed_analysis,
    impact_score       = excluded.impact_score,
    breaking_changes   = excluded.breaking_changes,
    packages_affected  = excluded.packages_affected`

	_, err = s.db.ExecContext(ctx, query,
		analysis.RepoPath, analysis.CommitHash, analysis.ParentHash,
		analysis.Author, analysis.AuthorEmail, analysis.CommitDate,
		analysis.Message, filesJSON, diffJSON, analysis.Category,
		analysis.Summary, analysis.DetailedAnalysis, analysis.ImpactScore,
		analysis.BreakingChanges, pkgsJSON,
	)
	if err != nil {
		return fmt.Errorf("save analysis: %w", err)
	}

	return nil
}

// GetAnalysis retrieves a single commit analysis by repo path and commit hash.
func (s *commitStore) GetAnalysis(ctx context.Context, repoPath, hash string) (*CommitAnalysis, error) {
	// Support both full and short (prefix) hash lookups.
	const exactQuery = `
SELECT id, repo_path, commit_hash, parent_hash, author, author_email,
       commit_date, message, files_changed, diff_stats, category,
       summary, detailed_analysis, impact_score, breaking_changes,
       packages_affected, created_at
FROM commit_analyses
WHERE repo_path = ? AND commit_hash = ?`

	const prefixQuery = `
SELECT id, repo_path, commit_hash, parent_hash, author, author_email,
       commit_date, message, files_changed, diff_stats, category,
       summary, detailed_analysis, impact_score, breaking_changes,
       packages_affected, created_at
FROM commit_analyses
WHERE repo_path = ? AND commit_hash LIKE ? || '%' ESCAPE '\'
LIMIT 1`

	var row *sql.Row
	if len(hash) >= 40 {
		row = s.db.QueryRowContext(ctx, exactQuery, repoPath, hash)
	} else if validate.IsHexString(hash) {
		// Short hex hash: use prefix LIKE lookup with escaped wildcards.
		row = s.db.QueryRowContext(ctx, prefixQuery, repoPath, validate.EscapeLikePattern(hash))
	} else {
		// Non-hex short string (e.g. branch name): try exact match only.
		row = s.db.QueryRowContext(ctx, exactQuery, repoPath, hash)
	}

	analysis, err := scanCommitAnalysis(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get analysis: %w", err)
	}

	return analysis, nil
}

// ListAnalyses returns analyses matching the provided filter.
func (s *commitStore) ListAnalyses(ctx context.Context, filter AnalysisFilter) ([]CommitAnalysis, error) {
	query, args := buildListQuery(filter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list analyses query: %w", err)
	}
	defer rows.Close()

	var results []CommitAnalysis
	for rows.Next() {
		analysis, err := scanCommitAnalysis(rows)
		if err != nil {
			return nil, fmt.Errorf("list analyses scan: %w", err)
		}
		results = append(results, *analysis)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list analyses rows: %w", err)
	}

	return results, nil
}

// IsAnalyzed returns true if an analysis exists for the given repo and commit hash.
func (s *commitStore) IsAnalyzed(ctx context.Context, repoPath, hash string) (bool, error) {
	const query = `SELECT 1 FROM commit_analyses WHERE repo_path = ? AND commit_hash = ? LIMIT 1`

	var exists int
	err := s.db.QueryRowContext(ctx, query, repoPath, hash).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("is analyzed: %w", err)
	}

	return true, nil
}

// CountByCategory returns the number of analyses per category for a repo.
func (s *commitStore) CountByCategory(ctx context.Context, repoPath string) (map[string]int, error) {
	const query = `
SELECT category, COUNT(*) AS cnt
FROM commit_analyses
WHERE repo_path = ?
GROUP BY category`

	rows, err := s.db.QueryContext(ctx, query, repoPath)
	if err != nil {
		return nil, fmt.Errorf("count by category: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, fmt.Errorf("count by category scan: %w", err)
		}
		counts[category] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count by category rows: %w", err)
	}

	return counts, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanCommitAnalysis(s scanner) (*CommitAnalysis, error) {
	var (
		a         CommitAnalysis
		filesJSON string
		diffJSON  string
		pkgsJSON  string
	)

	err := s.Scan(
		&a.ID, &a.RepoPath, &a.CommitHash, &a.ParentHash,
		&a.Author, &a.AuthorEmail, &a.CommitDate, &a.Message,
		&filesJSON, &diffJSON, &a.Category, &a.Summary,
		&a.DetailedAnalysis, &a.ImpactScore, &a.BreakingChanges,
		&pkgsJSON, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := unmarshalJSON(filesJSON, &a.FilesChanged); err != nil {
		return nil, fmt.Errorf("unmarshal files_changed: %w", err)
	}

	if err := unmarshalJSON(diffJSON, &a.DiffStats); err != nil {
		return nil, fmt.Errorf("unmarshal diff_stats: %w", err)
	}

	if err := unmarshalJSON(pkgsJSON, &a.PackagesAffected); err != nil {
		return nil, fmt.Errorf("unmarshal packages_affected: %w", err)
	}

	return &a, nil
}

func buildListQuery(f AnalysisFilter) (string, []any) {
	query := `
SELECT id, repo_path, commit_hash, parent_hash, author, author_email,
       commit_date, message, files_changed, diff_stats, category,
       summary, detailed_analysis, impact_score, breaking_changes,
       packages_affected, created_at
FROM commit_analyses
WHERE repo_path = ?`

	args := []any{f.RepoPath}

	if f.Category != "" {
		query += " AND category = ?"
		args = append(args, f.Category)
	}

	if f.Since != "" {
		query += " AND commit_date >= ?"
		args = append(args, f.Since)
	}

	if f.MinImpact > 0 {
		query += " AND impact_score >= ?"
		args = append(args, f.MinImpact)
	}

	query += " ORDER BY commit_date DESC"

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

func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshalJSON(s string, v any) error {
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), v)
}
