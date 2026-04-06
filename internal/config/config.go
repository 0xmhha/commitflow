package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	DBPath           string        `yaml:"db_path"`
	Model            string        `yaml:"model"`
	MaxDiffLines     int           `yaml:"max_diff_lines"`
	Delay            time.Duration `yaml:"delay"`
	Budget           float64       `yaml:"budget"`
	MaxBudgetPerCall float64       `yaml:"max_budget_per_call"`
	MaxRetries       int           `yaml:"max_retries"`
	RetryBackoff     time.Duration `yaml:"retry_backoff"`
	Verbose          bool          `yaml:"verbose"`
	DryRun           bool          `yaml:"dry_run"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	return Config{
		DBPath:           filepath.Join(home, ".local", "share", "git-upstream-tracker", "tracker.db"),
		Model:            "sonnet",
		MaxDiffLines:     5000,
		Delay:            1 * time.Second,
		Budget:           0,
		MaxBudgetPerCall: 0.50,
		MaxRetries:       3,
		RetryBackoff:     5 * time.Second,
		Verbose:          false,
		DryRun:           false,
	}
}

// DefaultConfigPath returns the default path for the YAML config file.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "git-upstream-tracker", "config.yaml")
}

// LoadConfig builds a Config by layering defaults, YAML file, and env vars.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	if err := mergeYAML(path, &cfg); err != nil {
		return Config{}, err
	}

	mergeEnv(&cfg)

	if err := ValidateConfig(cfg); err != nil {
		return Config{}, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// ValidateConfig checks that all configuration values are within sensible ranges.
func ValidateConfig(cfg Config) error {
	if cfg.MaxDiffLines < 0 {
		return fmt.Errorf("max_diff_lines must be non-negative, got %d", cfg.MaxDiffLines)
	}
	if cfg.Budget < 0 {
		return fmt.Errorf("budget must be non-negative, got %f", cfg.Budget)
	}
	if cfg.MaxBudgetPerCall < 0 {
		return fmt.Errorf("max_budget_per_call must be non-negative, got %f", cfg.MaxBudgetPerCall)
	}
	if cfg.MaxRetries < 0 || cfg.MaxRetries > 100 {
		return fmt.Errorf("max_retries must be between 0 and 100, got %d", cfg.MaxRetries)
	}
	if cfg.Delay < 0 {
		return fmt.Errorf("delay must be non-negative, got %v", cfg.Delay)
	}
	if cfg.RetryBackoff < 0 {
		return fmt.Errorf("retry_backoff must be non-negative, got %v", cfg.RetryBackoff)
	}
	return nil
}

// EnsureDBDir creates the parent directory of dbPath if it does not exist.
func EnsureDBDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create db directory %q: %w", dir, err)
	}
	return nil
}

// mergeYAML reads the YAML file at path and merges it into cfg.
// If the file does not exist, it is silently skipped.
func mergeYAML(path string, cfg *Config) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}

	return nil
}

// mergeEnv overrides cfg fields from environment variables when non-empty.
func mergeEnv(cfg *Config) {
	if v := os.Getenv("GUT_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("GUT_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("GUT_MAX_DIFF_LINES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxDiffLines = n
		}
	}
	if v := os.Getenv("GUT_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Delay = d
		}
	}
	if v := os.Getenv("GUT_BUDGET"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Budget = f
		}
	}
	if v := os.Getenv("GUT_VERBOSE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Verbose = b
		}
	}
	if v := os.Getenv("GUT_DRY_RUN"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.DryRun = b
		}
	}
}
