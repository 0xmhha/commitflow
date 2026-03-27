package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/0xmhha/commitflow/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	cfgFile      string
	verbose      bool
	dryRun       bool
	model        string
	dbPath       string
	maxDiffLines int
	delay        string
	budget       float64

	appConfig config.Config
)

// rootCmd is initialized at declaration so that init() functions in sibling
// files can safely call rootCmd.AddCommand without depending on init() ordering.
var rootCmd = &cobra.Command{
	Use:   "commitflow",
	Short: "Git commit analysis and upstream sync tool",
	Long: `commitflow analyzes git commits using AI and helps fork projects
selectively apply upstream changes.`,
	PersistentPreRunE: initConfig,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// SetContext propagates the given context to all cobra commands via the root command.
func SetContext(ctx context.Context) {
	rootCmd.SetContext(ctx)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", config.DefaultConfigPath(), "config file path")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "simulate actions without making changes")
	rootCmd.PersistentFlags().StringVar(&model, "model", "sonnet", "AI model to use")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db-path", "", "path to the SQLite database")
	rootCmd.PersistentFlags().IntVar(&maxDiffLines, "max-diff-lines", 5000, "maximum diff lines to send to AI")
	rootCmd.PersistentFlags().StringVar(&delay, "delay", "1s", "delay between API calls")
	rootCmd.PersistentFlags().Float64Var(&budget, "budget", 0, "total budget limit in USD (0 = unlimited)")
}

func initConfig(cmd *cobra.Command, _ []string) error {
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	appConfig = applyFlagOverrides(cmd.Root().PersistentFlags(), cfg)
	return nil
}

// applyFlagOverrides merges explicitly set CLI flags into cfg, returning a new Config.
func applyFlagOverrides(pf *pflag.FlagSet, cfg config.Config) config.Config {
	result := cfg
	if pf.Changed("verbose") {
		result.Verbose = verbose
	}
	if pf.Changed("dry-run") {
		result.DryRun = dryRun
	}
	if pf.Changed("model") {
		result.Model = model
	}
	if pf.Changed("db-path") {
		result.DBPath = dbPath
	}
	if pf.Changed("max-diff-lines") {
		result.MaxDiffLines = maxDiffLines
	}
	if pf.Changed("delay") {
		if d, err := time.ParseDuration(delay); err == nil {
			result.Delay = d
		}
	}
	if pf.Changed("budget") {
		result.Budget = budget
	}
	return result
}
