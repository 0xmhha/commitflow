package upstream

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xmhha/commitflow/internal/git"
	"github.com/0xmhha/commitflow/internal/storage"
	internalsync "github.com/0xmhha/commitflow/internal/sync"
	"github.com/spf13/cobra"
)

var applyName string

var applyCmd = &cobra.Command{
	Use:   "apply <commit-hash>",
	Short: "Apply an upstream commit to the fork",
	Long:  "Cherry-pick an upstream commit onto the fork repository in a new branch.",
	Args:  cobra.ExactArgs(1),
	RunE:  runApply,
}

func init() {
	applyCmd.Flags().StringVar(&applyName, "name", "", "upstream config name (required)")
	_ = applyCmd.MarkFlagRequired("name")
	UpstreamCmd.AddCommand(applyCmd)
}

func runApply(_ *cobra.Command, args []string) error {
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

	upstreamCfg, err := resolveUpstreamConfig(ctx, syncStore, applyName)
	if err != nil {
		return err
	}

	syncRecord, err := resolveSyncRecord(ctx, syncStore, upstreamCfg.ID, hash)
	if err != nil {
		return err
	}

	if err := validateApplyStatus(syncRecord.Status); err != nil {
		return err
	}

	forkRepo, err := git.NewRepository(upstreamCfg.ForkPath)
	if err != nil {
		return fmt.Errorf("open fork repository %q: %w", upstreamCfg.ForkPath, err)
	}

	applier := internalsync.NewApplier(forkRepo, syncStore, cfg.Verbose)
	result, err := applier.Apply(ctx, syncRecord)
	if err != nil && result == nil {
		return fmt.Errorf("apply commit %s: %w", shortHash(hash), err)
	}

	printApplyResult(hash, result)
	return nil
}

// resolveUpstreamConfig retrieves the upstream config by name, returning an error
// if it does not exist.
func resolveUpstreamConfig(ctx context.Context, syncStore storage.SyncStore, name string) (*storage.UpstreamConfig, error) {
	upstreamCfg, err := syncStore.GetConfig(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get upstream config %q: %w", name, err)
	}
	if upstreamCfg == nil {
		return nil, fmt.Errorf("upstream config %q not found; run 'tracker upstream init' first", name)
	}
	return upstreamCfg, nil
}

// resolveSyncRecord retrieves the sync record for a given config + hash pair.
func resolveSyncRecord(ctx context.Context, syncStore storage.SyncStore, configID int64, hash string) (*storage.UpstreamSync, error) {
	syncRecord, err := syncStore.GetSync(ctx, configID, hash)
	if err != nil {
		return nil, fmt.Errorf("get sync record for %s: %w", shortHash(hash), err)
	}
	if syncRecord == nil {
		return nil, fmt.Errorf("no sync record found for commit %s; run 'tracker upstream scan' first", shortHash(hash))
	}
	return syncRecord, nil
}

// validateApplyStatus ensures the sync record is in a state that allows applying.
func validateApplyStatus(status string) error {
	switch status {
	case "applicable", "needs_review":
		return nil
	case "applied":
		return fmt.Errorf("commit already applied")
	case "not_applicable":
		return fmt.Errorf("commit is marked not_applicable and cannot be applied")
	case "skipped":
		return fmt.Errorf("commit has been skipped; use a different command to force-apply")
	case "conflict":
		return fmt.Errorf("commit previously resulted in a conflict; resolve manually or use skip")
	default:
		return fmt.Errorf("unexpected status %q; expected applicable or needs_review", status)
	}
}

// printApplyResult prints the outcome of an apply operation to stdout.
func printApplyResult(hash string, result *internalsync.ApplyResult) {
	if result.Success {
		fmt.Printf("Applied %s as %s on branch upstream/%s\n",
			shortHash(hash),
			shortHash(result.AppliedCommit),
			shortHashN(hash, 8),
		)
		return
	}

	fileList := strings.Join(result.ConflictedFiles, ", ")
	if fileList == "" {
		fileList = "(unknown)"
	}
	fmt.Printf("Conflict in %d file(s): %s. Cherry-pick aborted.\n",
		len(result.ConflictedFiles),
		fileList,
	)
}

func shortHashN(hash string, n int) string {
	if len(hash) > n {
		return hash[:n]
	}
	return hash
}
