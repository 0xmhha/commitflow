package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/0xmhha/commitflow/internal/ai"
	"github.com/0xmhha/commitflow/internal/analysis"
	"github.com/0xmhha/commitflow/internal/config"
	"github.com/0xmhha/commitflow/internal/git"
	"github.com/0xmhha/commitflow/internal/storage"
	"github.com/spf13/cobra"
)

// aiResponseAdapter wraps *ai.Response to satisfy analysis.AIResponse.
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

// aiClientAdapter wraps *ai.Client to satisfy analysis.AIClient.
type aiClientAdapter struct {
	client *ai.Client
}

func (a *aiClientAdapter) Call(ctx context.Context, prompt string, jsonSchema string) (analysis.AIResponse, error) {
	resp, err := a.client.Call(ctx, prompt, jsonSchema)
	if err != nil {
		return nil, err
	}
	return &aiResponseAdapter{resp: resp}, nil
}

func (a *aiClientAdapter) AccumulatedCost() float64 {
	return a.client.AccumulatedCost()
}

var (
	analyzeFrom string
	analyzeTo   string
	analyzeLast int
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [repo-path]",
	Short: "Analyze git commits in a repository",
	Long:  "Analyze commits in a git repository using AI to classify and summarize changes.",
	Args:  cobra.ExactArgs(1),
	RunE:  runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVar(&analyzeFrom, "from", "", "starting commit hash or ref (exclusive)")
	analyzeCmd.Flags().StringVar(&analyzeTo, "to", "HEAD", "ending commit hash or ref (inclusive)")
	analyzeCmd.Flags().IntVar(&analyzeLast, "last", 0, "analyze the last N commits from HEAD")

	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	repoPath := args[0]

	if err := validateRepoPath(repoPath); err != nil {
		return err
	}

	if !appConfig.DryRun {
		if err := ai.CheckClaude(); err != nil {
			return fmt.Errorf("claude CLI check: %w", err)
		}
	}

	db, err := openDatabase(appConfig.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	engine, err := buildEngine(db, repoPath)
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, err := runEngineAnalysis(ctx, engine, cmd)
	if err != nil {
		return err
	}

	printAnalysisSummary(result)
	return nil
}

func validateRepoPath(path string) error {
	if path == "" {
		return fmt.Errorf("repo-path must not be empty")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("repo-path %q: %w", path, err)
	}
	return nil
}

func openDatabase(dbPath string) (*sql.DB, error) {
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

func buildEngine(db *sql.DB, repoPath string) (*analysis.Engine, error) {
	repo, err := git.NewRepository(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	store := storage.NewCommitStore(db)
	aiClient := ai.NewClient(ai.ClientConfig{
		Model:             appConfig.Model,
		MaxBudgetPerCall:  appConfig.MaxBudgetPerCall,
		TotalBudgetLimit:  appConfig.Budget,
		DelayBetweenCalls: appConfig.Delay,
		MaxRetries:        appConfig.MaxRetries,
		RetryBackoff:      appConfig.RetryBackoff,
		DryRun:            appConfig.DryRun,
		Verbose:           appConfig.Verbose,
	})

	engineCfg := analysis.EngineConfig{
		MaxDiffLines: appConfig.MaxDiffLines,
		Verbose:      appConfig.Verbose,
	}

	return analysis.NewEngine(repo, store, &aiClientAdapter{client: aiClient}, engineCfg), nil
}

func runEngineAnalysis(ctx context.Context, engine *analysis.Engine, cmd *cobra.Command) (*analysis.AnalysisResult, error) {
	if cmd.Flags().Changed("last") || analyzeLast > 0 {
		return engine.AnalyzeLast(ctx, analyzeLast)
	}
	return engine.AnalyzeRange(ctx, analyzeFrom, analyzeTo)
}

func printAnalysisSummary(result *analysis.AnalysisResult) {
	fmt.Printf("\nAnalysis complete:\n")
	fmt.Printf("  Total commits:    %d\n", result.TotalCommits)
	fmt.Printf("  Already analyzed: %d\n", result.AlreadyAnalyzed)
	fmt.Printf("  Newly analyzed:   %d\n", result.NewlyAnalyzed)
	fmt.Printf("  Skipped:          %d\n", result.Skipped)
	fmt.Printf("  Errors:           %d\n", result.Errors)
	fmt.Printf("  Total cost:       $%.4f\n", result.TotalCost)

	if len(result.CategoryCounts) > 0 {
		fmt.Printf("\nCategory breakdown:\n")
		for cat, count := range result.CategoryCounts {
			fmt.Printf("  %-20s %d\n", analysis.CategoryLabel(cat)+" "+cat, count)
		}
	}

	if len(result.ErrorDetails) > 0 {
		fmt.Fprintf(os.Stderr, "\nErrors encountered:\n")
		for _, e := range result.ErrorDetails {
			fmt.Fprintf(os.Stderr, "  [%s] %s: %v\n", e.Phase, e.CommitHash[:8], e.Err)
		}
	}
}
