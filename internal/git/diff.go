package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	// DefaultMaxDiffLines is the line budget below which the full diff is included.
	DefaultMaxDiffLines = 5000
	// MediumDiffThreshold is the line count above which only Go files are kept.
	MediumDiffThreshold = 20000
)

// DiffResult holds the outcome of a commit diff extraction.
type DiffResult struct {
	FullDiff       string
	StatSummary    string
	FileDiffs      []FileDiff
	TotalLines     int
	TotalAdditions int
	TotalDeletions int
	Truncated      bool
	TruncationNote string
}

// FileDiff holds per-file diff information.
type FileDiff struct {
	Path      string
	OldPath   string // Populated for renamed files.
	Additions int
	Deletions int
	Diff      string
	Binary    bool
}

// DiffOpts configures diff extraction behaviour.
type DiffOpts struct {
	MaxDiffLines int // 0 → DefaultMaxDiffLines
}

// GetCommitDiff returns the diff for a single commit, applying a 3-tier
// chunking strategy based on the total line count.
//
// Tier 1 (< maxDiffLines):   full diff included.
// Tier 2 (< MediumDiffThreshold): .go files only, rest summarised.
// Tier 3 (>= MediumDiffThreshold): stat summary only, marked as truncated.
func (r *Repository) GetCommitDiff(ctx context.Context, hash string, opts DiffOpts) (*DiffResult, error) {
	maxLines := opts.MaxDiffLines
	if maxLines <= 0 {
		maxLines = DefaultMaxDiffLines
	}

	parent, err := r.parentRef(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("get commit diff %q: %w", hash, err)
	}

	statSummary, err := r.diffStat(ctx, parent, hash)
	if err != nil {
		return nil, fmt.Errorf("get commit diff %q stat: %w", hash, err)
	}

	numstatOut, err := r.diffNumstat(ctx, parent, hash)
	if err != nil {
		return nil, fmt.Errorf("get commit diff %q numstat: %w", hash, err)
	}

	fileDiffs := parseNumstat(numstatOut)
	totalAdd, totalDel := sumStats(fileDiffs)
	totalLines := totalAdd + totalDel

	result := &DiffResult{
		StatSummary:    statSummary,
		FileDiffs:      fileDiffs,
		TotalLines:     totalLines,
		TotalAdditions: totalAdd,
		TotalDeletions: totalDel,
	}

	switch {
	case totalLines >= MediumDiffThreshold:
		result.Truncated = true
		result.TruncationNote = fmt.Sprintf(
			"Diff too large (%d lines). Showing stat summary only.", totalLines,
		)

	case totalLines >= maxLines:
		result.Truncated = true
		result.TruncationNote = fmt.Sprintf(
			"Diff exceeds %d lines (%d total). Showing .go files only.", maxLines, totalLines,
		)
		full, err := r.diffFull(ctx, parent, hash)
		if err != nil {
			return nil, fmt.Errorf("get commit diff %q full: %w", hash, err)
		}
		result.FullDiff = filterGoFiles(full)

	default:
		full, err := r.diffFull(ctx, parent, hash)
		if err != nil {
			return nil, fmt.Errorf("get commit diff %q full: %w", hash, err)
		}
		result.FullDiff = full
	}

	return result, nil
}

// GetConflictedFiles returns the paths of files with unresolved merge conflicts.
func (r *Repository) GetConflictedFiles(ctx context.Context) ([]string, error) {
	out, err := r.runGit(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, fmt.Errorf("get conflicted files: %w", err)
	}

	if out == "" {
		return []string{}, nil
	}

	return strings.Split(out, "\n"), nil
}

// parseNumstat converts the output of `git diff --numstat` into a []FileDiff.
// Each line has the format: "additions\tdeletions\tfilepath"
// Binary files are represented as "-\t-\tfilepath".
func parseNumstat(output string) []FileDiff {
	if output == "" {
		return []FileDiff{}
	}

	lines := strings.Split(output, "\n")
	result := make([]FileDiff, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fd := parseNumstatLine(line)
		if fd != nil {
			result = append(result, *fd)
		}
	}

	return result
}

// parseNumstatLine parses a single --numstat line into a FileDiff.
func parseNumstatLine(line string) *FileDiff {
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) != 3 {
		return nil
	}

	addStr := strings.TrimSpace(parts[0])
	delStr := strings.TrimSpace(parts[1])
	rawPath := strings.TrimSpace(parts[2])

	fd := &FileDiff{}

	if addStr == "-" && delStr == "-" {
		fd.Binary = true
	} else {
		add, err := strconv.Atoi(addStr)
		if err == nil {
			fd.Additions = add
		}
		del, err := strconv.Atoi(delStr)
		if err == nil {
			fd.Deletions = del
		}
	}

	// Handle renames: "old => new" or "{old/path => new/path}/suffix"
	if strings.Contains(rawPath, " => ") {
		oldPath, newPath := splitRenamePath(rawPath)
		fd.OldPath = oldPath
		fd.Path = newPath
	} else {
		fd.Path = rawPath
	}

	return fd
}

// splitRenamePath handles git rename notation such as
// "src/{old => new}/file.go" or simply "old.go => new.go".
func splitRenamePath(raw string) (oldPath, newPath string) {
	// Brace-style rename: "prefix/{old => new}/suffix"
	open := strings.Index(raw, "{")
	close := strings.Index(raw, "}")
	if open != -1 && close != -1 && open < close {
		prefix := raw[:open]
		suffix := raw[close+1:]
		inner := raw[open+1 : close]

		parts := strings.SplitN(inner, " => ", 2)
		if len(parts) == 2 {
			return prefix + parts[0] + suffix, prefix + parts[1] + suffix
		}
	}

	// Simple rename: "old => new"
	parts := strings.SplitN(raw, " => ", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}

	return raw, raw
}

// parentRef returns a diff-safe ref for the parent of hash.
// For a root commit (no parent) it returns the empty tree hash.
func (r *Repository) parentRef(ctx context.Context, hash string) (string, error) {
	out, err := r.runGit(ctx, "rev-parse", "--verify", hash+"^")
	if err != nil {
		// Root commit: diff against the empty tree.
		return "4b825dc642cb6eb9a060e54bf8d69288fbee4904", nil
	}
	return out, nil
}

func (r *Repository) diffStat(ctx context.Context, parent, hash string) (string, error) {
	return r.runGit(ctx, "diff", "--stat", parent+".."+hash)
}

func (r *Repository) diffNumstat(ctx context.Context, parent, hash string) (string, error) {
	return r.runGit(ctx, "diff", "--numstat", parent+".."+hash)
}

func (r *Repository) diffFull(ctx context.Context, parent, hash string) (string, error) {
	return r.runGit(ctx, "diff", parent+".."+hash)
}

// sumStats returns the total additions and deletions across all FileDiffs.
func sumStats(files []FileDiff) (totalAdd, totalDel int) {
	for _, f := range files {
		totalAdd += f.Additions
		totalDel += f.Deletions
	}
	return totalAdd, totalDel
}

// filterGoFiles retains only hunks from .go files in the raw unified diff text.
func filterGoFiles(fullDiff string) string {
	if fullDiff == "" {
		return ""
	}

	var sb strings.Builder
	var inGoFile bool

	for _, line := range strings.Split(fullDiff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			inGoFile = strings.HasSuffix(line, ".go")
		}
		if inGoFile {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
