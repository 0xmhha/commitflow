package upstream

import (
	"context"
	"fmt"

	"github.com/0xmhha/commitflow/internal/storage"
	"github.com/spf13/cobra"
)

var (
	skipName   string
	skipReason string
)

var skipCmd = &cobra.Command{
	Use:   "skip <commit-hash>",
	Short: "Skip an upstream commit",
	Long:  "Mark an upstream commit as deliberately skipped so it will not appear as pending.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkip,
}

func init() {
	skipCmd.Flags().StringVar(&skipName, "name", "", "upstream config name (required)")
	skipCmd.Flags().StringVar(&skipReason, "reason", "", "reason for skipping (required)")
	_ = skipCmd.MarkFlagRequired("name")
	_ = skipCmd.MarkFlagRequired("reason")
	UpstreamCmd.AddCommand(skipCmd)
}

func runSkip(_ *cobra.Command, args []string) error {
	hash := args[0]
	if hash == "" {
		return fmt.Errorf("commit-hash must not be empty")
	}

	cfg := GetConfig()
	ctx := context.Background()

	db, err := openDB(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	syncStore := storage.NewSyncStore(db)

	upstreamCfg, err := resolveUpstreamConfig(ctx, syncStore, skipName)
	if err != nil {
		return err
	}

	syncRecord, err := resolveSyncRecord(ctx, syncStore, upstreamCfg.ID, hash)
	if err != nil {
		return err
	}

	if err := validateSkipStatus(syncRecord.Status); err != nil {
		return err
	}

	if err := syncStore.MarkSkipped(ctx, syncRecord.ID, skipReason); err != nil {
		return fmt.Errorf("mark skipped %s: %w", shortHash(hash), err)
	}

	fmt.Printf("Skipped %s: %s\n", shortHash(hash), skipReason)
	return nil
}

// validateSkipStatus ensures the sync record is not already in a terminal state
// that prevents skipping.
func validateSkipStatus(status string) error {
	switch status {
	case "applied":
		return fmt.Errorf("commit is already applied and cannot be skipped")
	case "skipped":
		return fmt.Errorf("commit is already skipped")
	default:
		return nil
	}
}
