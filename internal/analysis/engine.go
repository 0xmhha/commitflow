package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/0xmhha/commitflow/internal/ai"
	"github.com/0xmhha/commitflow/internal/git"
	"github.com/0xmhha/commitflow/internal/storage"
)

// AIClient is the interface the Engine requires from the AI layer.
// Defining it here avoids a direct compile-time dependency on the concrete
// ai.Client type and makes the engine straightforward to test with a mock.
type AIClient interface {
	Call(ctx context.Context, prompt string, jsonSchema string) (AIResponse, error)
	AccumulatedCost() float64
}

// AIResponse is the subset of the AI response that the engine reads.
type AIResponse interface {
	GetStructuredOutput() []byte
	GetCostUSD() float64
	GetInputTokens() int
	GetOutputTokens() int
	GetDurationMS() int
	IsErr() bool
}

// EngineConfig holds tuneable parameters for an Engine instance.
type EngineConfig struct {
	MaxDiffLines int
	Verbose      bool
}

// AnalysisResult summarises a completed analysis run.
type AnalysisResult struct {
	TotalCommits    int
	AlreadyAnalyzed int
	NewlyAnalyzed   int
	Skipped         int
	Errors          int
	TotalCost       float64
	CategoryCounts  map[string]int
	ErrorDetails    []AnalysisError
}

// AnalysisError captures a per-commit failure with the phase that failed.
type AnalysisError struct {
	CommitHash string
	Phase      string // "metadata" | "diff" | "ai" | "storage"
	Err        error
}

// Error implements the error interface so AnalysisError can be returned as error.
func (e AnalysisError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying cause so errors.Is/As works through the wrapper.
func (e AnalysisError) Unwrap() error {
	return e.Err
}

// Engine orchestrates commit analysis using the injected dependencies.
type Engine struct {
	repo         *git.Repository
	store        storage.CommitStore
	aiClient     AIClient
	maxDiffLines int
	verbose      bool
}

// NewEngine constructs an Engine with the provided dependencies and config.
func NewEngine(
	repo *git.Repository,
	store storage.CommitStore,
	aiClient AIClient,
	cfg EngineConfig,
) *Engine {
	maxLines := cfg.MaxDiffLines
	if maxLines <= 0 {
		maxLines = git.DefaultMaxDiffLines
	}
	return &Engine{
		repo:         repo,
		store:        store,
		aiClient:     aiClient,
		maxDiffLines: maxLines,
		verbose:      cfg.Verbose,
	}
}

// AnalyzeRange analyzes commits in the half-open range (from, to].
func (e *Engine) AnalyzeRange(ctx context.Context, from, to string) (*AnalysisResult, error) {
	opts := git.CommitListOpts{From: from, To: to, Reverse: true}
	hashes, err := e.repo.ListCommits(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("analyze range: list commits: %w", err)
	}
	return e.analyzeHashes(ctx, hashes)
}

// AnalyzeLast analyzes the last n commits reachable from HEAD.
func (e *Engine) AnalyzeLast(ctx context.Context, n int) (*AnalysisResult, error) {
	opts := git.CommitListOpts{Last: n, Reverse: true}
	hashes, err := e.repo.ListCommits(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("analyze last %d: list commits: %w", n, err)
	}
	return e.analyzeHashes(ctx, hashes)
}

// analyzeHashes filters already-analyzed commits, prints progress, then
// processes each remaining commit in order.
func (e *Engine) analyzeHashes(ctx context.Context, hashes []string) (*AnalysisResult, error) {
	result := &AnalysisResult{
		TotalCommits:   len(hashes),
		CategoryCounts: make(map[string]int),
	}

	pending, alreadyDone, err := e.partitionHashes(ctx, hashes)
	if err != nil {
		return nil, err
	}
	result.AlreadyAnalyzed = alreadyDone

	fmt.Fprintf(os.Stderr, "Found %d commits, %d already analyzed, %d to analyze\n",
		len(hashes), alreadyDone, len(pending))

	for i, hash := range pending {
		if ctx.Err() != nil {
			break
		}
		e.processCommit(ctx, hash, i+1, len(pending), result)
	}

	return result, nil
}

// partitionHashes splits hashes into not-yet-analyzed (pending) and the count
// of already-analyzed commits.
func (e *Engine) partitionHashes(ctx context.Context, hashes []string) ([]string, int, error) {
	pending := make([]string, 0, len(hashes))
	alreadyDone := 0
	repoPath := e.repo.Path()

	for _, h := range hashes {
		analyzed, err := e.store.IsAnalyzed(ctx, repoPath, h)
		if err != nil {
			return nil, 0, fmt.Errorf("check analyzed %s: %w", h, err)
		}
		if analyzed {
			alreadyDone++
		} else {
			pending = append(pending, h)
		}
	}
	return pending, alreadyDone, nil
}

// processCommit runs the full analysis pipeline for a single commit and
// updates result in-place.
func (e *Engine) processCommit(
	ctx context.Context,
	hash string,
	idx, total int,
	result *AnalysisResult,
) {
	commit, diff, promptStr, buildErr := e.buildPromptForCommit(ctx, hash)
	if buildErr != nil {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, *buildErr)
		return
	}

	output, callErr := e.callAI(ctx, hash, promptStr)
	if callErr != nil {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, *callErr)
		return
	}

	// dry-run returns nil output; count as skipped, don't store.
	if output == nil {
		result.Skipped++
		fmt.Fprintf(os.Stderr, "[%d/%d] %s => [DRY-RUN] skipped\n", idx, total, hash[:minLen(len(hash), 8)])
		return
	}

	storageErr := e.storeResult(ctx, hash, commit, diff, output)
	if storageErr != nil {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, *storageErr)
		return
	}

	result.NewlyAnalyzed++
	result.TotalCost += e.aiClient.AccumulatedCost()
	result.CategoryCounts[output.Category]++

	fmt.Fprintf(os.Stderr, "[%d/%d] %s => %s %s (impact: %d/10) - %s\n",
		idx, total,
		hash[:8],
		CategoryLabel(output.Category),
		output.Category,
		output.ImpactScore,
		truncateMessage(commit.Subject, 60),
	)
}

// buildPromptForCommit fetches commit metadata and diff, then assembles the
// prompt. Returns a *AnalysisError (as the last return value) on failure so
// the caller can record it uniformly without a type assertion.
func (e *Engine) buildPromptForCommit(
	ctx context.Context,
	hash string,
) (*git.Commit, *git.DiffResult, string, *AnalysisError) {
	commit, err := e.repo.GetCommit(ctx, hash)
	if err != nil {
		return nil, nil, "", &AnalysisError{
			CommitHash: hash, Phase: "metadata",
			Err: fmt.Errorf("analyze commit %s: %w", hash, err),
		}
	}

	diff, err := e.repo.GetCommitDiff(ctx, hash, git.DiffOpts{MaxDiffLines: e.maxDiffLines})
	if err != nil {
		return nil, nil, "", &AnalysisError{
			CommitHash: hash, Phase: "diff",
			Err: fmt.Errorf("analyze commit %s: %w", hash, err),
		}
	}

	filesChanged := collectFilePaths(diff.FileDiffs)
	message := strings.TrimSpace(commit.Subject + "\n" + commit.Body)

	data := PromptData{
		Hash:           commit.Hash,
		Author:         commit.Author,
		AuthorEmail:    commit.AuthorEmail,
		Date:           commit.Date,
		Message:        message,
		FilesChanged:   filesChanged,
		Additions:      diff.TotalAdditions,
		Deletions:      diff.TotalDeletions,
		Diff:           diff.FullDiff,
		DiffTruncated:  diff.Truncated,
		TruncationNote: diff.TruncationNote,
		TotalDiffLines: diff.TotalLines,
		StatSummary:    diff.StatSummary,
	}

	prompt, err := BuildAnalysisPrompt(data)
	if err != nil {
		return nil, nil, "", &AnalysisError{
			CommitHash: hash, Phase: "ai",
			Err: fmt.Errorf("analyze commit %s: build prompt: %w", hash, err),
		}
	}

	return commit, diff, prompt, nil
}

// callAI invokes the AI client and parses the structured output. Returns a
// pointer to AnalysisError on failure so it is distinguishable from nil.
func (e *Engine) callAI(
	ctx context.Context,
	hash string,
	prompt string,
) (*ai.CommitAnalysisOutput, *AnalysisError) {
	resp, err := e.aiClient.Call(ctx, prompt, ai.CommitAnalysisSchema)
	if err != nil {
		return nil, &AnalysisError{
			CommitHash: hash, Phase: "ai",
			Err: fmt.Errorf("analyze commit %s: ai call: %w", hash, err),
		}
	}
	if resp.IsErr() {
		return nil, &AnalysisError{
			CommitHash: hash, Phase: "ai",
			Err: fmt.Errorf("analyze commit %s: ai returned error", hash),
		}
	}

	raw := resp.GetStructuredOutput()
	if len(raw) == 0 {
		return nil, nil
	}

	var output ai.CommitAnalysisOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, &AnalysisError{
			CommitHash: hash, Phase: "ai",
			Err: fmt.Errorf("analyze commit %s: parse ai output: %w", hash, err),
		}
	}
	if err := ai.ValidateCommitAnalysis(&output); err != nil {
		return nil, &AnalysisError{
			CommitHash: hash, Phase: "ai",
			Err: fmt.Errorf("analyze commit %s: invalid ai output: %w", hash, err),
		}
	}
	return &output, nil
}

// storeResult maps CommitAnalysisOutput to storage.CommitAnalysis and saves it.
func (e *Engine) storeResult(
	ctx context.Context,
	hash string,
	commit *git.Commit,
	diff *git.DiffResult,
	output *ai.CommitAnalysisOutput,
) *AnalysisError {
	record := &storage.CommitAnalysis{
		RepoPath:    e.repo.Path(),
		CommitHash:  commit.Hash,
		ParentHash:  commit.ParentHash,
		Author:      commit.Author,
		AuthorEmail: commit.AuthorEmail,
		CommitDate:  commit.Date,
		Message:     strings.TrimSpace(commit.Subject + "\n" + commit.Body),
		FilesChanged: collectFilePaths(diff.FileDiffs),
		DiffStats: storage.DiffStats{
			Additions:  diff.TotalAdditions,
			Deletions:  diff.TotalDeletions,
			FilesCount: len(diff.FileDiffs),
		},
		Category:         output.Category,
		Summary:          output.Summary,
		DetailedAnalysis: output.DetailedAnalysis,
		ImpactScore:      output.ImpactScore,
		BreakingChanges:  output.BreakingChanges,
		PackagesAffected: output.PackagesAffected,
	}

	if err := e.store.SaveAnalysis(ctx, record); err != nil {
		return &AnalysisError{
			CommitHash: hash, Phase: "storage",
			Err: fmt.Errorf("analyze commit %s: save: %w", hash, err),
		}
	}
	return nil
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// collectFilePaths extracts the Path of every FileDiff into a string slice.
func collectFilePaths(fileDiffs []git.FileDiff) []string {
	paths := make([]string, 0, len(fileDiffs))
	for _, fd := range fileDiffs {
		paths = append(paths, fd.Path)
	}
	return paths
}

// truncateMessage shortens s to at most maxLen runes, appending "..." when cut.
func truncateMessage(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
