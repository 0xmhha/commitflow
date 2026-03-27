package upstream

import (
	"context"
	"fmt"

	"github.com/0xmhha/commitflow/internal/git"
	"github.com/0xmhha/commitflow/internal/storage"
	"github.com/spf13/cobra"
)

var (
	initUpstreamURL string
	initForkPath    string
	initName        string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize upstream tracking",
	Long:  "Initialize tracking for an upstream repository against a local fork.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVar(&initUpstreamURL, "upstream", "", "URL of the upstream repository (required)")
	initCmd.Flags().StringVar(&initForkPath, "fork", "", "local path to the fork repository (required)")
	initCmd.Flags().StringVar(&initName, "name", "", "name for this upstream configuration (required)")

	_ = initCmd.MarkFlagRequired("upstream")
	_ = initCmd.MarkFlagRequired("fork")
	_ = initCmd.MarkFlagRequired("name")
	UpstreamCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	cfg := GetConfig()
	ctx := context.Background()

	forkRepo, err := git.NewRepository(initForkPath)
	if err != nil {
		return fmt.Errorf("open fork repository: %w", err)
	}

	if err := forkRepo.AddRemote(ctx, "upstream", initUpstreamURL); err != nil {
		return fmt.Errorf("add upstream remote: %w", err)
	}

	fmt.Printf("Fetching upstream %s...\n", initUpstreamURL)
	if err := forkRepo.Fetch(ctx, "upstream"); err != nil {
		return fmt.Errorf("fetch upstream: %w", err)
	}

	mergeBase, err := findMergeBase(ctx, forkRepo)
	if err != nil {
		return fmt.Errorf("find merge base: %w", err)
	}

	db, err := openDB(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	syncStore := storage.NewSyncStore(db)
	upstreamCfg := &storage.UpstreamConfig{
		Name:           initName,
		UpstreamURL:    initUpstreamURL,
		ForkPath:       initForkPath,
		LastSyncedHash: mergeBase,
	}

	if err := syncStore.SaveConfig(ctx, upstreamCfg); err != nil {
		return fmt.Errorf("save upstream config: %w", err)
	}

	fmt.Printf("Initialized. Fork diverged from upstream at %s\n", shortHash(mergeBase))
	return nil
}

// findMergeBase tries upstream/main then upstream/master as the upstream branch.
func findMergeBase(ctx context.Context, forkRepo *git.Repository) (string, error) {
	for _, branch := range []string{"upstream/main", "upstream/master"} {
		base, err := forkRepo.MergeBase(ctx, branch, "HEAD")
		if err == nil && base != "" {
			return base, nil
		}
	}
	return "", fmt.Errorf("could not determine merge base: tried upstream/main and upstream/master")
}

func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}
