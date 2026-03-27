package upstream

import (
	"context"
	"fmt"

	"github.com/0xmhha/commitflow/internal/ai"
	"github.com/0xmhha/commitflow/internal/git"
	"github.com/0xmhha/commitflow/internal/storage"
	internalsync "github.com/0xmhha/commitflow/internal/sync"
	"github.com/spf13/cobra"
)

var (
	scanName string
	scanFrom string
	scanTo   string
	scanLast int
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan upstream commits for applicability",
	Long:  "Scan upstream commits and assess whether each is applicable to the fork.",
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().StringVar(&scanName, "name", "", "upstream config name (required)")
	scanCmd.Flags().StringVar(&scanFrom, "from", "", "starting commit hash or ref (exclusive)")
	scanCmd.Flags().StringVar(&scanTo, "to", "HEAD", "ending commit hash or ref (inclusive)")
	scanCmd.Flags().IntVar(&scanLast, "last", 0, "scan the last N upstream commits")

	_ = scanCmd.MarkFlagRequired("name")
	UpstreamCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, _ []string) error {
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

	upstreamCfg, err := syncStore.GetConfig(ctx, scanName)
	if err != nil {
		return fmt.Errorf("get upstream config %q: %w", scanName, err)
	}
	if upstreamCfg == nil {
		return fmt.Errorf("upstream config %q not found; run 'tracker upstream init' first", scanName)
	}

	// Both upstreamRepo and forkRepo point to the fork directory because
	// upstream is accessed as a git remote ("upstream") within the fork repo.
	repo, err := git.NewRepository(upstreamCfg.ForkPath)
	if err != nil {
		return fmt.Errorf("open fork repository: %w", err)
	}

	aiClient := buildAIClient(cfg)
	trackerCfg := internalsync.TrackerConfig{
		MaxDiffLines: cfg.MaxDiffLines,
		Verbose:      cfg.Verbose,
	}

	tracker := internalsync.NewTracker(
		repo,
		repo,
		syncStore,
		commitStore,
		aiClient,
		upstreamCfg,
		trackerCfg,
	)

	result, err := runTrackerScan(ctx, tracker, cmd)
	if err != nil {
		return err
	}

	printScanSummary(result)
	return nil
}

func runTrackerScan(ctx context.Context, tracker *internalsync.Tracker, cmd *cobra.Command) (*internalsync.ScanResult, error) {
	if cmd.Flags().Changed("last") || scanLast > 0 {
		return tracker.ScanLast(ctx, scanLast)
	}
	return tracker.ScanRange(ctx, scanFrom, scanTo)
}

func printScanSummary(result *internalsync.ScanResult) {
	fmt.Printf("\nScan complete:\n")
	fmt.Printf("  Total commits:    %d\n", result.TotalCommits)
	fmt.Printf("  Already scanned:  %d\n", result.AlreadyScanned)
	fmt.Printf("  Applicable:       %d\n", result.Applicable)
	fmt.Printf("  Not applicable:   %d\n", result.NotApplicable)
	fmt.Printf("  Needs review:     %d\n", result.NeedsReview)
	fmt.Printf("  Already applied:  %d\n", result.AlreadyApplied)
	fmt.Printf("  Errors:           %d\n", result.Errors)
	fmt.Printf("  Total cost:       $%.4f\n", result.TotalCost)
}
