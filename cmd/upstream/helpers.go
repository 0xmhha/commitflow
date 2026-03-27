package upstream

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/0xmhha/commitflow/internal/ai"
	"github.com/0xmhha/commitflow/internal/config"
	"github.com/0xmhha/commitflow/internal/storage"
	internalsync "github.com/0xmhha/commitflow/internal/sync"
)

// GetConfig returns the current application config provided by the parent cmd package.
// It is set via SetConfig when the upstream subcommand is registered.
var getConfigFn func() config.Config

// SetConfigProvider registers the function used to retrieve the current config.
// This must be called by the cmd package during startup to avoid import cycles.
func SetConfigProvider(fn func() config.Config) {
	getConfigFn = fn
}

// GetConfig returns the resolved application configuration.
func GetConfig() config.Config {
	if getConfigFn != nil {
		return getConfigFn()
	}
	return config.DefaultConfig()
}

// openDB opens and migrates the SQLite database at the given path.
func openDB(dbPath string) (*sql.DB, error) {
	if err := config.EnsureDBDir(dbPath); err != nil {
		return nil, fmt.Errorf("ensure db directory: %w", err)
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := storage.RunMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return db, nil
}

// buildAIClient constructs an ai.Client wrapped to satisfy the sync.AIClient interface.
func buildAIClient(cfg config.Config) internalsync.AIClient {
	client := ai.NewClient(ai.ClientConfig{
		Model:             cfg.Model,
		MaxBudgetPerCall:  cfg.MaxBudgetPerCall,
		TotalBudgetLimit:  cfg.Budget,
		DelayBetweenCalls: cfg.Delay,
		MaxRetries:        cfg.MaxRetries,
		RetryBackoff:      cfg.RetryBackoff,
		DryRun:            cfg.DryRun,
		Verbose:           cfg.Verbose,
	})
	return &aiClientAdapter{client: client}
}

// aiClientAdapter wraps *ai.Client to satisfy sync.AIClient.
type aiClientAdapter struct {
	client *ai.Client
}

func (a *aiClientAdapter) Call(ctx context.Context, prompt string, jsonSchema string) (internalsync.AIResponse, error) {
	resp, err := a.client.Call(ctx, prompt, jsonSchema)
	if err != nil {
		return nil, err
	}
	return &aiResponseAdapter{resp: resp}, nil
}

func (a *aiClientAdapter) AccumulatedCost() float64 {
	return a.client.AccumulatedCost()
}

// aiResponseAdapter wraps *ai.Response to satisfy sync.AIResponse.
type aiResponseAdapter struct {
	resp *ai.Response
}

func (a *aiResponseAdapter) GetStructuredOutput() []byte {
	if a.resp == nil {
		return nil
	}
	return a.resp.StructuredOutput
}

func (a *aiResponseAdapter) GetCostUSD() float64 {
	if a.resp == nil {
		return 0
	}
	return a.resp.TotalCostUSD
}

func (a *aiResponseAdapter) GetInputTokens() int {
	if a.resp == nil {
		return 0
	}
	return a.resp.Usage.InputTokens
}

func (a *aiResponseAdapter) GetOutputTokens() int {
	if a.resp == nil {
		return 0
	}
	return a.resp.Usage.OutputTokens
}

func (a *aiResponseAdapter) GetDurationMS() int {
	if a.resp == nil {
		return 0
	}
	return a.resp.DurationMS
}

func (a *aiResponseAdapter) IsErr() bool {
	if a.resp == nil {
		return false
	}
	return a.resp.IsError
}
