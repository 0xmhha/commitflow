package storage

import (
	"context"
	"testing"
)

func TestCommitStore_SaveAndGet(t *testing.T) {
	db := testDB(t)
	store := NewCommitStore(db)
	ctx := context.Background()

	analysis := &CommitAnalysis{
		RepoPath:         "/test/repo",
		CommitHash:       "abc123def456789012345678901234567890abcd",
		ParentHash:       "parent123",
		Author:           "Test Author",
		AuthorEmail:      "test@example.com",
		CommitDate:       "2025-03-15",
		Message:          "Fix bug",
		FilesChanged:     []string{"main.go", "util.go"},
		DiffStats:        DiffStats{Additions: 10, Deletions: 5, FilesCount: 2},
		Category:         "bug_fix",
		Summary:          "Fixed a memory leak",
		DetailedAnalysis: "Detailed explanation here",
		ImpactScore:      7,
		BreakingChanges:  false,
		PackagesAffected: []string{"pkg/core"},
	}

	if err := store.SaveAnalysis(ctx, analysis); err != nil {
		t.Fatalf("SaveAnalysis: %v", err)
	}

	// Full hash lookup.
	got, err := store.GetAnalysis(ctx, "/test/repo", "abc123def456789012345678901234567890abcd")
	if err != nil {
		t.Fatalf("GetAnalysis (full hash): %v", err)
	}
	if got == nil {
		t.Fatal("GetAnalysis returned nil")
	}
	if got.CommitHash != analysis.CommitHash {
		t.Errorf("CommitHash = %q, want %q", got.CommitHash, analysis.CommitHash)
	}
	if got.Category != "bug_fix" {
		t.Errorf("Category = %q, want %q", got.Category, "bug_fix")
	}
	if got.ImpactScore != 7 {
		t.Errorf("ImpactScore = %d, want 7", got.ImpactScore)
	}
	if len(got.FilesChanged) != 2 {
		t.Errorf("FilesChanged len = %d, want 2", len(got.FilesChanged))
	}
	if got.DiffStats.Additions != 10 {
		t.Errorf("DiffStats.Additions = %d, want 10", got.DiffStats.Additions)
	}
}

func TestCommitStore_GetAnalysis_ShortHash(t *testing.T) {
	db := testDB(t)
	store := NewCommitStore(db)
	ctx := context.Background()

	analysis := &CommitAnalysis{
		RepoPath:         "/test/repo",
		CommitHash:       "abc123def456789012345678901234567890abcd",
		Category:         "feature",
		FilesChanged:     []string{},
		PackagesAffected: []string{},
	}

	if err := store.SaveAnalysis(ctx, analysis); err != nil {
		t.Fatalf("SaveAnalysis: %v", err)
	}

	// Short hash (prefix) lookup.
	got, err := store.GetAnalysis(ctx, "/test/repo", "abc123de")
	if err != nil {
		t.Fatalf("GetAnalysis (short hash): %v", err)
	}
	if got == nil {
		t.Fatal("GetAnalysis with short hash returned nil")
	}
	if got.Category != "feature" {
		t.Errorf("Category = %q, want %q", got.Category, "feature")
	}
}

func TestCommitStore_GetAnalysis_NotFound(t *testing.T) {
	db := testDB(t)
	store := NewCommitStore(db)
	ctx := context.Background()

	got, err := store.GetAnalysis(ctx, "/test/repo", "nonexistent")
	if err != nil {
		t.Fatalf("GetAnalysis: %v", err)
	}
	if got != nil {
		t.Error("GetAnalysis for nonexistent hash should return nil")
	}
}

func TestCommitStore_IsAnalyzed(t *testing.T) {
	db := testDB(t)
	store := NewCommitStore(db)
	ctx := context.Background()

	analysis := &CommitAnalysis{
		RepoPath:         "/test/repo",
		CommitHash:       "abc123def456789012345678901234567890abcd",
		Category:         "chore",
		FilesChanged:     []string{},
		PackagesAffected: []string{},
	}

	// Before save.
	analyzed, err := store.IsAnalyzed(ctx, "/test/repo", "abc123def456789012345678901234567890abcd")
	if err != nil {
		t.Fatalf("IsAnalyzed before save: %v", err)
	}
	if analyzed {
		t.Error("IsAnalyzed before save = true, want false")
	}

	// After save.
	if err := store.SaveAnalysis(ctx, analysis); err != nil {
		t.Fatalf("SaveAnalysis: %v", err)
	}

	analyzed, err = store.IsAnalyzed(ctx, "/test/repo", "abc123def456789012345678901234567890abcd")
	if err != nil {
		t.Fatalf("IsAnalyzed after save: %v", err)
	}
	if !analyzed {
		t.Error("IsAnalyzed after save = false, want true")
	}
}

func TestCommitStore_ListAnalyses_WithFilters(t *testing.T) {
	db := testDB(t)
	store := NewCommitStore(db)
	ctx := context.Background()

	analyses := []*CommitAnalysis{
		{RepoPath: "/repo", CommitHash: "hash1", Category: "bug_fix", ImpactScore: 8, CommitDate: "2025-03-01", FilesChanged: []string{}, PackagesAffected: []string{}},
		{RepoPath: "/repo", CommitHash: "hash2", Category: "feature", ImpactScore: 3, CommitDate: "2025-03-05", FilesChanged: []string{}, PackagesAffected: []string{}},
		{RepoPath: "/repo", CommitHash: "hash3", Category: "bug_fix", ImpactScore: 9, CommitDate: "2025-03-10", FilesChanged: []string{}, PackagesAffected: []string{}},
		{RepoPath: "/repo", CommitHash: "hash4", Category: "security", ImpactScore: 10, CommitDate: "2025-03-15", FilesChanged: []string{}, PackagesAffected: []string{}},
	}

	for _, a := range analyses {
		if err := store.SaveAnalysis(ctx, a); err != nil {
			t.Fatalf("SaveAnalysis %s: %v", a.CommitHash, err)
		}
	}

	// Filter by category.
	results, err := store.ListAnalyses(ctx, AnalysisFilter{RepoPath: "/repo", Category: "bug_fix"})
	if err != nil {
		t.Fatalf("ListAnalyses category filter: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("category filter returned %d results, want 2", len(results))
	}

	// Filter by min impact.
	results, err = store.ListAnalyses(ctx, AnalysisFilter{RepoPath: "/repo", MinImpact: 9})
	if err != nil {
		t.Fatalf("ListAnalyses impact filter: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("impact filter returned %d results, want 2", len(results))
	}

	// Filter by since date.
	results, err = store.ListAnalyses(ctx, AnalysisFilter{RepoPath: "/repo", Since: "2025-03-10"})
	if err != nil {
		t.Fatalf("ListAnalyses since filter: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("since filter returned %d results, want 2", len(results))
	}

	// Limit.
	results, err = store.ListAnalyses(ctx, AnalysisFilter{RepoPath: "/repo", Limit: 2})
	if err != nil {
		t.Fatalf("ListAnalyses limit: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("limit returned %d results, want 2", len(results))
	}
}

func TestCommitStore_CountByCategory(t *testing.T) {
	db := testDB(t)
	store := NewCommitStore(db)
	ctx := context.Background()

	analyses := []*CommitAnalysis{
		{RepoPath: "/repo", CommitHash: "h1", Category: "bug_fix", FilesChanged: []string{}, PackagesAffected: []string{}},
		{RepoPath: "/repo", CommitHash: "h2", Category: "bug_fix", FilesChanged: []string{}, PackagesAffected: []string{}},
		{RepoPath: "/repo", CommitHash: "h3", Category: "feature", FilesChanged: []string{}, PackagesAffected: []string{}},
	}

	for _, a := range analyses {
		if err := store.SaveAnalysis(ctx, a); err != nil {
			t.Fatalf("SaveAnalysis: %v", err)
		}
	}

	counts, err := store.CountByCategory(ctx, "/repo")
	if err != nil {
		t.Fatalf("CountByCategory: %v", err)
	}

	if counts["bug_fix"] != 2 {
		t.Errorf("bug_fix count = %d, want 2", counts["bug_fix"])
	}
	if counts["feature"] != 1 {
		t.Errorf("feature count = %d, want 1", counts["feature"])
	}
}

func TestCommitStore_SaveAnalysis_Upsert(t *testing.T) {
	db := testDB(t)
	store := NewCommitStore(db)
	ctx := context.Background()

	analysis := &CommitAnalysis{
		RepoPath:         "/repo",
		CommitHash:       "hash1",
		Category:         "bug_fix",
		Summary:          "Original",
		FilesChanged:     []string{},
		PackagesAffected: []string{},
	}

	if err := store.SaveAnalysis(ctx, analysis); err != nil {
		t.Fatalf("first SaveAnalysis: %v", err)
	}

	// Update same commit.
	analysis.Category = "security"
	analysis.Summary = "Updated"
	if err := store.SaveAnalysis(ctx, analysis); err != nil {
		t.Fatalf("second SaveAnalysis (upsert): %v", err)
	}

	got, err := store.GetAnalysis(ctx, "/repo", "hash1")
	if err != nil {
		t.Fatalf("GetAnalysis after upsert: %v", err)
	}
	if got.Category != "security" {
		t.Errorf("Category after upsert = %q, want %q", got.Category, "security")
	}
	if got.Summary != "Updated" {
		t.Errorf("Summary after upsert = %q, want %q", got.Summary, "Updated")
	}
}
