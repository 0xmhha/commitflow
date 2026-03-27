package upstream

import (
	"context"
	"fmt"

	"github.com/0xmhha/commitflow/internal/storage"
	"github.com/spf13/cobra"
)

var (
	statusName       string
	statusShowPending int
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show upstream sync status",
	Long:  "Display the current sync status for an upstream tracking configuration.",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusName, "name", "", "upstream config name (required)")
	statusCmd.Flags().IntVar(&statusShowPending, "pending", 0, "show next N pending commits with relevance scores")

	_ = statusCmd.MarkFlagRequired("name")
	UpstreamCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, _ []string) error {
	cfg := GetConfig()
	ctx := context.Background()

	db, err := openDB(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	syncStore := storage.NewSyncStore(db)

	upstreamCfg, err := syncStore.GetConfig(ctx, statusName)
	if err != nil {
		return fmt.Errorf("get upstream config %q: %w", statusName, err)
	}
	if upstreamCfg == nil {
		return fmt.Errorf("upstream config %q not found; run 'tracker upstream init' first", statusName)
	}

	counts, err := syncStore.CountByStatus(ctx, upstreamCfg.ID)
	if err != nil {
		return fmt.Errorf("count sync statuses: %w", err)
	}

	printStatus(upstreamCfg, counts)

	if statusShowPending > 0 {
		if err := printPendingCommits(ctx, syncStore, upstreamCfg.ID, statusShowPending); err != nil {
			return err
		}
	}

	return nil
}

func printStatus(cfg *storage.UpstreamConfig, counts map[string]int) {
	lastSynced := cfg.LastSyncedHash
	if lastSynced == "" {
		lastSynced = "(none)"
	} else {
		lastSynced = shortHash(lastSynced)
	}

	fmt.Printf("Upstream: %s (%s)\n", cfg.Name, cfg.UpstreamURL)
	fmt.Printf("Fork:     %s\n", cfg.ForkPath)
	fmt.Printf("Last synced: %s\n", lastSynced)
	fmt.Printf("\nStatus breakdown:\n")
	fmt.Printf("  %-18s %d\n", "Pending:", counts["pending"])
	fmt.Printf("  %-18s %d\n", "Applicable:", counts["applicable"])
	fmt.Printf("  %-18s %d\n", "Not applicable:", counts["not_applicable"])
	fmt.Printf("  %-18s %d\n", "Applied:", counts["applied"])
	fmt.Printf("  %-18s %d\n", "Skipped:", counts["skipped"])
	fmt.Printf("  %-18s %d\n", "Conflicts:", counts["conflict"])
	fmt.Printf("  %-18s %d\n", "Needs review:", counts["needs_review"])
}

func printPendingCommits(ctx context.Context, syncStore storage.SyncStore, configID int64, limit int) error {
	filter := storage.SyncFilter{
		ConfigID: configID,
		Status:   "pending",
		Limit:    limit,
	}

	syncs, err := syncStore.ListSyncs(ctx, filter)
	if err != nil {
		return fmt.Errorf("list pending commits: %w", err)
	}

	if len(syncs) == 0 {
		return nil
	}

	fmt.Printf("\nNext %d pending commits:\n", len(syncs))
	fmt.Printf("  %-14s  %-5s  %s\n", "Commit", "Score", "Reason")
	fmt.Printf("  %-14s  %-5s  %s\n", "------", "-----", "------")

	for _, s := range syncs {
		reason := s.ApplicabilityReason
		if reason == "" {
			reason = "(not yet assessed)"
		}
		if len(reason) > 60 {
			reason = reason[:57] + "..."
		}
		fmt.Printf("  %-14s  %-5d  %s\n", shortHash(s.UpstreamCommit), s.RelevanceScore, reason)
	}

	return nil
}
