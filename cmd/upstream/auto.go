package upstream

import (
	"context"
	"fmt"

	"github.com/0xmhha/commitflow/internal/ai"
	"github.com/0xmhha/commitflow/internal/config"
	"github.com/0xmhha/commitflow/internal/git"
	"github.com/0xmhha/commitflow/internal/storage"
	internalsync "github.com/0xmhha/commitflow/internal/sync"
	"github.com/spf13/cobra"
)

var (
	autoName string
	autoFrom string
	autoTo   string
	autoLast int
)

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Automatically scan and apply upstream commits",
	Long:  "Run a full scan of upstream commits and attempt to apply all applicable ones in order.",
	RunE:  runAuto,
}

func init() {
	autoCmd.Flags().StringVar(&autoName, "name", "", "upstream config name (required)")
	autoCmd.Flags().StringVar(&autoFrom, "from", "", "starting commit hash or ref (exclusive)")
	autoCmd.Flags().StringVar(&autoTo, "to", "HEAD", "ending commit hash or ref (inclusive)")
	autoCmd.Flags().IntVar(&autoLast, "last", 0, "scan the last N upstream commits")
	_ = autoCmd.MarkFlagRequired("name")
	UpstreamCmd.AddCommand(autoCmd)
}

func runAuto(cmd *cobra.Command, _ []string) error {
	cfg := GetConfig()
	ctx := context.Background()

	if !cfg.DryRun {
		if err := ai.CheckClaude(); err != nil {
			return fmt.Errorf("claude CLI check: %w", err)
		}
	}

	db, err := openDB(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	syncStore := storage.NewSyncStore(db)
	commitStore := storage.NewCommitStore(db)

	upstreamCfg, err := resolveUpstreamConfig(ctx, syncStore, autoName)
	if err != nil {
		return err
	}

	scanResult, err := executeScan(ctx, cmd, syncStore, commitStore, upstreamCfg, cfg)
	if err != nil {
		return fmt.Errorf("scan phase: %w", err)
	}
	printScanSummary(scanResult)

	stats, err := executeApplyPhase(ctx, syncStore, upstreamCfg, cfg.Verbose)
	if err != nil {
		return fmt.Errorf("apply phase: %w", err)
	}
	printAutoSummary(stats)

	return nil
}

// executeScan builds a Tracker and runs the appropriate scan strategy.
func executeScan(
	ctx context.Context,
	cmd *cobra.Command,
	syncStore storage.SyncStore,
	commitStore storage.CommitStore,
	upstreamCfg *storage.UpstreamConfig,
	cfg config.Config,
) (*internalsync.ScanResult, error) {
	upstreamRepo, err := git.NewRepository(upstreamCfg.ForkPath)
	if err != nil {
		return nil, fmt.Errorf("open upstream repository: %w", err)
	}

	forkRepo, err := git.NewRepository(upstreamCfg.ForkPath)
	if err != nil {
		return nil, fmt.Errorf("open fork repository: %w", err)
	}

	aiClient := buildAIClient(cfg)
	trackerCfg := internalsync.TrackerConfig{
		MaxDiffLines: cfg.MaxDiffLines,
		Verbose:      cfg.Verbose,
	}

	tracker := internalsync.NewTracker(
		upstreamRepo,
		forkRepo,
		syncStore,
		commitStore,
		aiClient,
		upstreamCfg,
		trackerCfg,
	)

	if cmd.Flags().Changed("last") || autoLast > 0 {
		return tracker.ScanLast(ctx, autoLast)
	}
	return tracker.ScanRange(ctx, autoFrom, autoTo)
}

// autoStats tracks the outcome counters for the apply phase.
type autoStats struct {
	applied   int
	conflicts int
	skipped   int
}

// executeApplyPhase fetches all "applicable" syncs and cherry-picks each in
// order. Conflicts are counted and do not halt the pipeline.
func executeApplyPhase(
	ctx context.Context,
	syncStore storage.SyncStore,
	upstreamCfg *storage.UpstreamConfig,
	verbose bool,
) (autoStats, error) {
	filter := storage.SyncFilter{
		ConfigID: upstreamCfg.ID,
		Status:   "applicable",
	}

	syncs, err := syncStore.ListSyncs(ctx, filter)
	if err != nil {
		return autoStats{}, fmt.Errorf("list applicable commits: %w", err)
	}

	if len(syncs) == 0 {
		return autoStats{}, nil
	}

	forkRepo, err := git.NewRepository(upstreamCfg.ForkPath)
	if err != nil {
		return autoStats{}, fmt.Errorf("open fork repository: %w", err)
	}

	applier := internalsync.NewApplier(forkRepo, syncStore, verbose)
	stats := autoStats{}

	for i := range syncs {
		if ctx.Err() != nil {
			break
		}
		s := &syncs[i]
		result, err := applier.Apply(ctx, s)
		if err != nil && result == nil {
			return stats, fmt.Errorf("apply %s: %w", shortHash(s.UpstreamCommit), err)
		}
		if result.Success {
			stats.applied++
		} else {
			stats.conflicts++
		}
	}

	return stats, nil
}

func printAutoSummary(stats autoStats) {
	fmt.Printf("\nAuto pipeline complete:\n")
	fmt.Printf("  Applied:    %d\n", stats.applied)
	fmt.Printf("  Conflicts:  %d\n", stats.conflicts)
	fmt.Printf("  Skipped:    %d\n", stats.skipped)
}
