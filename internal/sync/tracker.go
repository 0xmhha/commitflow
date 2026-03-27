package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/0xmhha/commitflow/internal/ai"
	"github.com/0xmhha/commitflow/internal/git"
	"github.com/0xmhha/commitflow/internal/storage"
)

// AIClient is the interface the Tracker requires from the AI layer.
// Using a local interface avoids a hard compile-time dependency on the
// concrete ai.Client type and keeps the tracker testable with mocks.
type AIClient interface {
	Call(ctx context.Context, prompt string, jsonSchema string) (AIResponse, error)
	AccumulatedCost() float64
}

// AIResponse is the subset of the AI response the tracker reads.
type AIResponse interface {
	GetStructuredOutput() []byte
	GetCostUSD() float64
	GetInputTokens() int
	GetOutputTokens() int
	GetDurationMS() int
	IsErr() bool
}

// TrackerConfig holds tuneable parameters for a Tracker instance.
type TrackerConfig struct {
	MaxDiffLines int
	Verbose      bool
}

// ScanResult summarises a completed scan run.
type ScanResult struct {
	TotalCommits   int
	AlreadyScanned int
	Applicable     int
	NotApplicable  int
	NeedsReview    int
	AlreadyApplied int
	Errors         int
	TotalCost      float64
}

// Tracker orchestrates upstream commit scanning and applicability assessment.
type Tracker struct {
	upstreamRepo *git.Repository
	forkRepo     *git.Repository
	syncStore    storage.SyncStore
	commitStore  storage.CommitStore
	aiClient     AIClient
	config       *storage.UpstreamConfig
	maxDiffLines int
	verbose      bool
}

// NewTracker constructs a Tracker with the provided dependencies.
func NewTracker(
	upstreamRepo, forkRepo *git.Repository,
	syncStore storage.SyncStore,
	commitStore storage.CommitStore,
	aiClient AIClient,
	cfg *storage.UpstreamConfig,
	trackerCfg TrackerConfig,
) *Tracker {
	maxLines := trackerCfg.MaxDiffLines
	if maxLines <= 0 {
		maxLines = git.DefaultMaxDiffLines
	}
	return &Tracker{
		upstreamRepo: upstreamRepo,
		forkRepo:     forkRepo,
		syncStore:    syncStore,
		commitStore:  commitStore,
		aiClient:     aiClient,
		config:       cfg,
		maxDiffLines: maxLines,
		verbose:      trackerCfg.Verbose,
	}
}

// ScanRange scans upstream commits in the half-open range (from, to] and
// assesses applicability for each previously unseen commit.
func (t *Tracker) ScanRange(ctx context.Context, from, to string) (*ScanResult, error) {
	opts := git.CommitListOpts{From: from, To: to, Reverse: true}
	hashes, err := t.upstreamRepo.ListCommits(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("scan upstream range: list commits: %w", err)
	}
	return t.scanHashes(ctx, hashes)
}

// ScanLast scans the last n upstream commits reachable from HEAD.
func (t *Tracker) ScanLast(ctx context.Context, n int) (*ScanResult, error) {
	opts := git.CommitListOpts{Last: n, Reverse: true}
	hashes, err := t.upstreamRepo.ListCommits(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("scan upstream last %d: list commits: %w", n, err)
	}
	return t.scanHashes(ctx, hashes)
}

// scanHashes processes each commit hash in order, skipping already-scanned
// commits and recording the result in the sync store.
func (t *Tracker) scanHashes(ctx context.Context, hashes []string) (*ScanResult, error) {
	result := &ScanResult{TotalCommits: len(hashes)}

	pending, alreadyScanned, err := t.partitionHashes(ctx, hashes)
	if err != nil {
		return nil, err
	}
	result.AlreadyScanned = alreadyScanned

	if t.verbose {
		slog.Info("upstream scan",
			"total", len(hashes),
			"already_scanned", alreadyScanned,
			"pending", len(pending),
		)
	}

	var lastHash string
	for _, hash := range pending {
		if ctx.Err() != nil {
			break
		}
		status := t.processHash(ctx, hash, result)
		if status != "" {
			lastHash = hash
		}
	}

	if lastHash != "" {
		if err := t.syncStore.UpdateLastSyncedHash(ctx, t.config.ID, lastHash); err != nil {
			return result, fmt.Errorf("update last synced hash: %w", err)
		}
	}

	return result, nil
}

// partitionHashes splits hashes into unseen (pending) and returns the count
// of already-scanned commits.
func (t *Tracker) partitionHashes(ctx context.Context, hashes []string) ([]string, int, error) {
	pending := make([]string, 0, len(hashes))
	alreadyScanned := 0

	for _, h := range hashes {
		synced, err := t.syncStore.IsSynced(ctx, t.config.ID, h)
		if err != nil {
			return nil, 0, fmt.Errorf("check synced %s: %w", h, err)
		}
		if synced {
			alreadyScanned++
		} else {
			pending = append(pending, h)
		}
	}

	return pending, alreadyScanned, nil
}

// processHash runs the full scan pipeline for a single commit.
// Returns the final status string or an empty string on error.
func (t *Tracker) processHash(ctx context.Context, hash string, result *ScanResult) string {
	diffResult, err := t.upstreamRepo.GetCommitDiff(ctx, hash, git.DiffOpts{MaxDiffLines: t.maxDiffLines})
	if err != nil {
		result.Errors++
		slog.Warn("get upstream diff failed", "hash", hash[:8], "error", err)
		return ""
	}

	filesChanged := collectFilePaths(diffResult.FileDiffs)

	overlapping, _ := CheckFileOverlap(ctx, t.forkRepo, filesChanged)
	if len(overlapping) == 0 && len(filesChanged) > 0 {
		return t.recordNoOverlap(ctx, hash, result)
	}

	return t.assessWithAI(ctx, hash, diffResult, result)
}

// recordNoOverlap saves a not_applicable record when no files overlap with the fork.
func (t *Tracker) recordNoOverlap(ctx context.Context, hash string, result *ScanResult) string {
	const reason = "No overlapping files between upstream commit and fork"
	sync := &storage.UpstreamSync{
		ConfigID:            t.config.ID,
		UpstreamCommit:      hash,
		Status:              "not_applicable",
		ApplicabilityReason: reason,
	}
	if err := t.syncStore.SaveSync(ctx, sync); err != nil {
		result.Errors++
		slog.Warn("save no-overlap sync failed", "hash", hash[:8], "error", err)
		return ""
	}
	result.NotApplicable++
	return "not_applicable"
}

// assessWithAI calls the AI for an applicability assessment and stores the result.
func (t *Tracker) assessWithAI(
	ctx context.Context,
	hash string,
	diffResult *git.DiffResult,
	result *ScanResult,
) string {
	commit, err := t.upstreamRepo.GetCommit(ctx, hash)
	if err != nil {
		result.Errors++
		slog.Warn("get upstream commit failed", "hash", hash[:8], "error", err)
		return ""
	}

	existingAnalysis, _ := t.commitStore.GetAnalysis(ctx, t.upstreamRepo.Path(), hash)

	forkCtx, err := GatherForkContext(ctx, t.forkRepo, collectFilePaths(diffResult.FileDiffs))
	if err != nil {
		result.Errors++
		slog.Warn("gather fork context failed", "hash", hash[:8], "error", err)
		return ""
	}

	divergencePoint, _ := t.forkRepo.MergeBase(ctx, "HEAD", "HEAD")

	data := buildApplicabilityData(commit, diffResult, existingAnalysis, forkCtx, divergencePoint)

	prompt, err := BuildApplicabilityPrompt(data)
	if err != nil {
		result.Errors++
		slog.Warn("build applicability prompt failed", "hash", hash[:8], "error", err)
		return ""
	}

	aiResp, err := t.aiClient.Call(ctx, prompt, ai.ApplicabilitySchema)
	if err != nil {
		result.Errors++
		slog.Warn("AI call failed", "hash", hash[:8], "error", err)
		return ""
	}

	if aiResp.IsErr() {
		result.Errors++
		slog.Warn("AI returned error", "hash", hash[:8])
		return ""
	}

	var output ai.ApplicabilityOutput
	if err := json.Unmarshal(aiResp.GetStructuredOutput(), &output); err != nil {
		result.Errors++
		slog.Warn("parse AI output failed", "hash", hash[:8], "error", err)
		return ""
	}

	if err := t.storeAssessment(ctx, hash, &output, aiResp); err != nil {
		result.Errors++
		slog.Warn("store assessment failed", "hash", hash[:8], "error", err)
		return ""
	}

	updateScanResultCounts(result, output.Status, t.aiClient.AccumulatedCost())

	if t.verbose {
		slog.Info("assessed commit",
			"hash", hash[:8],
			"status", output.Status,
			"score", output.RelevanceScore,
		)
	}

	return output.Status
}

// storeAssessment saves the AI result and the call log to the sync store.
func (t *Tracker) storeAssessment(
	ctx context.Context,
	hash string,
	output *ai.ApplicabilityOutput,
	aiResp AIResponse,
) error {
	sync := &storage.UpstreamSync{
		ConfigID:            t.config.ID,
		UpstreamCommit:      hash,
		Status:              output.Status,
		ApplicabilityReason: output.Reason,
		RelevanceScore:      output.RelevanceScore,
	}
	if err := t.syncStore.SaveSync(ctx, sync); err != nil {
		return fmt.Errorf("save sync: %w", err)
	}

	callLog := &storage.CallLog{
		CallType:     "applicability",
		CommitHash:   hash,
		InputTokens:  aiResp.GetInputTokens(),
		OutputTokens: aiResp.GetOutputTokens(),
		CostUSD:      aiResp.GetCostUSD(),
		DurationMS:   aiResp.GetDurationMS(),
	}
	if err := t.syncStore.SaveCallLog(ctx, callLog); err != nil {
		return fmt.Errorf("save call log: %w", err)
	}

	return nil
}

// buildApplicabilityData assembles an ApplicabilityData from commit and fork context.
func buildApplicabilityData(
	commit *git.Commit,
	diffResult *git.DiffResult,
	analysis *storage.CommitAnalysis,
	forkCtx *ForkContext,
	divergencePoint string,
) ApplicabilityData {
	message := strings.TrimSpace(commit.Subject + "\n" + commit.Body)

	category := ""
	impactScore := 0
	var packages []string
	if analysis != nil {
		category = analysis.Category
		impactScore = analysis.ImpactScore
		packages = analysis.PackagesAffected
	}

	return ApplicabilityData{
		UpstreamHash:     commit.Hash,
		Message:          message,
		Category:         category,
		ImpactScore:      impactScore,
		Packages:         packages,
		Diff:             diffResult.FullDiff,
		StatSummary:      diffResult.StatSummary,
		DivergencePoint:  divergencePoint,
		ForkOnlyPackages: forkCtx.ForkOnlyDirs,
		OverlappingPkgs:  forkCtx.OverlappingFiles,
		MissingPkgs:      forkCtx.NonOverlappingFiles,
	}
}

// updateScanResultCounts increments the appropriate counter based on AI status.
func updateScanResultCounts(result *ScanResult, status string, accumulatedCost float64) {
	result.TotalCost = accumulatedCost
	switch status {
	case "applicable":
		result.Applicable++
	case "not_applicable":
		result.NotApplicable++
	case "needs_review":
		result.NeedsReview++
	case "already_applied":
		result.AlreadyApplied++
	}
}

// collectFilePaths extracts the Path of each FileDiff into a string slice.
func collectFilePaths(fileDiffs []git.FileDiff) []string {
	paths := make([]string, 0, len(fileDiffs))
	for _, fd := range fileDiffs {
		paths = append(paths, fd.Path)
	}
	return paths
}
