package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", cfg.Model, "sonnet")
	}
	if cfg.MaxDiffLines != 5000 {
		t.Errorf("MaxDiffLines = %d, want 5000", cfg.MaxDiffLines)
	}
	if cfg.Delay != 1*time.Second {
		t.Errorf("Delay = %v, want 1s", cfg.Delay)
	}
	if cfg.Budget != 0 {
		t.Errorf("Budget = %f, want 0", cfg.Budget)
	}
	if cfg.MaxBudgetPerCall != 0.50 {
		t.Errorf("MaxBudgetPerCall = %f, want 0.50", cfg.MaxBudgetPerCall)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.RetryBackoff != 5*time.Second {
		t.Errorf("RetryBackoff = %v, want 5s", cfg.RetryBackoff)
	}
	if cfg.Verbose {
		t.Error("Verbose = true, want false")
	}
	if cfg.DryRun {
		t.Error("DryRun = true, want false")
	}
	// DBPath should end with tracker.db
	if filepath.Base(cfg.DBPath) != "tracker.db" {
		t.Errorf("DBPath base = %q, want %q", filepath.Base(cfg.DBPath), "tracker.db")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("DefaultConfigPath() base = %q, want %q", filepath.Base(path), "config.yaml")
	}
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error for non-existent config file: %v", err)
	}
	// Should return defaults when file doesn't exist.
	if cfg.Model != "sonnet" {
		t.Errorf("Model = %q, want default %q", cfg.Model, "sonnet")
	}
}

func TestLoadConfig_EmptyPath(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "sonnet" {
		t.Errorf("Model = %q, want default %q", cfg.Model, "sonnet")
	}
}

func TestLoadConfig_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
model: opus
max_diff_lines: 10000
verbose: true
budget: 5.0
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Model != "opus" {
		t.Errorf("Model = %q, want %q", cfg.Model, "opus")
	}
	if cfg.MaxDiffLines != 10000 {
		t.Errorf("MaxDiffLines = %d, want 10000", cfg.MaxDiffLines)
	}
	if !cfg.Verbose {
		t.Error("Verbose = false, want true")
	}
	if cfg.Budget != 5.0 {
		t.Errorf("Budget = %f, want 5.0", cfg.Budget)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Use a tab character at start of line which is invalid YAML syntax.
	if err := os.WriteFile(path, []byte("key: value\n\t- broken"), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestMergeEnv(t *testing.T) {
	cfg := DefaultConfig()

	t.Setenv("GUT_MODEL", "opus")
	t.Setenv("GUT_MAX_DIFF_LINES", "8000")
	t.Setenv("GUT_DELAY", "3s")
	t.Setenv("GUT_BUDGET", "25.0")
	t.Setenv("GUT_VERBOSE", "true")
	t.Setenv("GUT_DRY_RUN", "true")
	t.Setenv("GUT_DB_PATH", "/tmp/test.db")

	mergeEnv(&cfg)

	if cfg.Model != "opus" {
		t.Errorf("Model = %q, want %q", cfg.Model, "opus")
	}
	if cfg.MaxDiffLines != 8000 {
		t.Errorf("MaxDiffLines = %d, want 8000", cfg.MaxDiffLines)
	}
	if cfg.Delay != 3*time.Second {
		t.Errorf("Delay = %v, want 3s", cfg.Delay)
	}
	if cfg.Budget != 25.0 {
		t.Errorf("Budget = %f, want 25.0", cfg.Budget)
	}
	if !cfg.Verbose {
		t.Error("Verbose = false, want true")
	}
	if !cfg.DryRun {
		t.Error("DryRun = false, want true")
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/tmp/test.db")
	}
}

func TestMergeEnv_InvalidValues(t *testing.T) {
	cfg := DefaultConfig()

	t.Setenv("GUT_MAX_DIFF_LINES", "not_a_number")
	t.Setenv("GUT_DELAY", "invalid")
	t.Setenv("GUT_BUDGET", "not_float")
	t.Setenv("GUT_VERBOSE", "not_bool")

	mergeEnv(&cfg)

	// Should keep defaults when env values are invalid.
	if cfg.MaxDiffLines != 5000 {
		t.Errorf("MaxDiffLines = %d, want default 5000", cfg.MaxDiffLines)
	}
	if cfg.Delay != 1*time.Second {
		t.Errorf("Delay = %v, want default 1s", cfg.Delay)
	}
}

func TestEnsureDBDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "deep", "tracker.db")

	if err := EnsureDBDir(dbPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parent directory should now exist.
	parentDir := filepath.Dir(dbPath)
	if _, err := os.Stat(parentDir); err != nil {
		t.Errorf("parent directory should exist: %v", err)
	}
}
