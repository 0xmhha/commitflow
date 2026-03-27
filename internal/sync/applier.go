package sync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/0xmhha/commitflow/internal/git"
	"github.com/0xmhha/commitflow/internal/storage"
)

// ApplyResult holds the outcome of a single cherry-pick attempt.
type ApplyResult struct {
	Success         bool
	AppliedCommit   string   // New commit hash in the fork on success.
	ConflictedFiles []string // Populated when cherry-pick results in conflicts.
	Error           error
}

// Applier executes cherry-pick operations on a fork repository.
type Applier struct {
	forkRepo  *git.Repository
	syncStore storage.SyncStore
	verbose   bool
}

// NewApplier constructs an Applier with the given dependencies.
func NewApplier(forkRepo *git.Repository, syncStore storage.SyncStore, verbose bool) *Applier {
	return &Applier{
		forkRepo:  forkRepo,
		syncStore: syncStore,
		verbose:   verbose,
	}
}

// Apply attempts to cherry-pick the upstream commit referenced by syncRecord
// onto the fork repository.
//
// Workflow:
//  1. Create branch: upstream/<hash[:8]>
//  2. Cherry-pick the upstream commit hash
//  3. Success: record new HEAD hash and mark the sync as applied
//  4. Conflict: collect conflicted files, abort cherry-pick, mark as conflict
func (a *Applier) Apply(ctx context.Context, syncRecord *storage.UpstreamSync) (*ApplyResult, error) {
	if syncRecord == nil {
		return nil, fmt.Errorf("apply: sync record must not be nil")
	}

	hash := syncRecord.UpstreamCommit
	branchName := branchNameForHash(hash)

	if err := a.forkRepo.CreateBranch(ctx, branchName); err != nil {
		return &ApplyResult{Error: err}, fmt.Errorf("apply: create branch %q: %w", branchName, err)
	}

	if a.verbose {
		slog.Info("cherry-picking upstream commit", "hash", hash[:8], "branch", branchName)
	}

	cherryPickErr := a.forkRepo.CherryPick(ctx, hash)
	if cherryPickErr == nil {
		return a.handleSuccess(ctx, syncRecord)
	}

	return a.handleConflict(ctx, syncRecord, cherryPickErr)
}

// handleSuccess records a successful cherry-pick result.
func (a *Applier) handleSuccess(
	ctx context.Context,
	syncRecord *storage.UpstreamSync,
) (*ApplyResult, error) {
	newHead, err := a.forkRepo.GetHead(ctx)
	if err != nil {
		return &ApplyResult{Error: err}, fmt.Errorf("apply: get HEAD after cherry-pick: %w", err)
	}

	if err := a.syncStore.MarkApplied(ctx, syncRecord.ID, newHead); err != nil {
		return &ApplyResult{
			Success:       true,
			AppliedCommit: newHead,
		}, fmt.Errorf("apply: mark applied in store: %w", err)
	}

	if a.verbose {
		slog.Info("cherry-pick succeeded",
			"upstream", syncRecord.UpstreamCommit[:8],
			"fork_commit", newHead[:8],
		)
	}

	return &ApplyResult{
		Success:       true,
		AppliedCommit: newHead,
	}, nil
}

// handleConflict aborts the cherry-pick and records the conflict state.
func (a *Applier) handleConflict(
	ctx context.Context,
	syncRecord *storage.UpstreamSync,
	cherryPickErr error,
) (*ApplyResult, error) {
	conflicted, listErr := a.forkRepo.GetConflictedFiles(ctx)
	if listErr != nil {
		slog.Warn("could not list conflicted files", "error", listErr)
		conflicted = []string{}
	}

	if abortErr := a.forkRepo.AbortCherryPick(ctx); abortErr != nil {
		slog.Warn("cherry-pick abort failed", "error", abortErr)
	}

	reason := fmt.Sprintf("cherry-pick conflict: %v", cherryPickErr)
	if err := a.syncStore.UpdateSyncStatus(ctx, syncRecord.ID, "conflict", reason); err != nil {
		slog.Warn("update sync status to conflict failed", "error", err)
	}

	if a.verbose {
		slog.Info("cherry-pick conflict",
			"upstream", syncRecord.UpstreamCommit[:8],
			"conflicted_files", len(conflicted),
		)
	}

	return &ApplyResult{
		Success:         false,
		ConflictedFiles: conflicted,
		Error:           cherryPickErr,
	}, nil
}

// branchNameForHash returns a deterministic branch name for an upstream commit.
func branchNameForHash(hash string) string {
	short := hash
	if len(hash) > 8 {
		short = hash[:8]
	}
	return fmt.Sprintf("upstream/%s", short)
}
