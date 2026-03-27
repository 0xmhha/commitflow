package git

import (
	"context"
	"fmt"
	"strings"
)

// Commit holds the metadata for a single git commit.
type Commit struct {
	Hash        string
	ParentHash  string
	Author      string
	AuthorEmail string
	Date        string
	Subject     string
	Body        string
}

// CommitListOpts controls which commits are returned by ListCommits.
// Either set Last > 0 for the last N commits, or set From/To for a range.
type CommitListOpts struct {
	From    string // Start hash (exclusive). Used with To.
	To      string // End hash (inclusive). Used with From.
	Last    int    // Last N commits (alternative to From/To).
	Reverse bool   // Return commits in chronological (oldest-first) order.
}

// ListCommits returns the commit hashes selected by opts.
// When opts.Last > 0 it runs: git rev-list [-N] [--reverse] HEAD
// Otherwise it runs:          git rev-list [--reverse] <From>..<To>
func (r *Repository) ListCommits(ctx context.Context, opts CommitListOpts) ([]string, error) {
	args := []string{"rev-list"}

	if opts.Reverse {
		args = append(args, "--reverse")
	}

	if opts.Last > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", opts.Last), "HEAD")
	} else {
		args = append(args, fmt.Sprintf("%s..%s", opts.From, opts.To))
	}

	out, err := r.runGit(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("list commits: %w", err)
	}

	if out == "" {
		return []string{}, nil
	}

	return strings.Split(out, "\n"), nil
}

// GetCommit fetches the full metadata for a single commit hash.
// The format string uses NUL-separated fields to safely handle multi-line
// body content.
func (r *Repository) GetCommit(ctx context.Context, hash string) (*Commit, error) {
	// Use %x00 (NUL) as field separator to avoid conflicts with newlines in
	// the commit body. We request exactly 7 fields.
	const format = "%H%x00%P%x00%an%x00%ae%x00%aI%x00%s%x00%b"

	out, err := r.runGit(ctx, "log", "-1", "--format="+format, hash)
	if err != nil {
		return nil, fmt.Errorf("get commit %q: %w", hash, err)
	}

	return parseCommit(out)
}

// parseCommit splits the NUL-delimited log output into a Commit struct.
func parseCommit(raw string) (*Commit, error) {
	// TrimSpace may strip the trailing NUL so we split on NUL directly.
	parts := strings.SplitN(raw, "\x00", 7)
	if len(parts) < 7 {
		return nil, fmt.Errorf("parse commit: unexpected format, got %d fields", len(parts))
	}

	// A merge commit has multiple parent hashes space-separated; we store only
	// the first to keep the type simple.
	parentHash := strings.Fields(parts[1])
	firstParent := ""
	if len(parentHash) > 0 {
		firstParent = parentHash[0]
	}

	return &Commit{
		Hash:        strings.TrimSpace(parts[0]),
		ParentHash:  firstParent,
		Author:      strings.TrimSpace(parts[2]),
		AuthorEmail: strings.TrimSpace(parts[3]),
		Date:        strings.TrimSpace(parts[4]),
		Subject:     strings.TrimSpace(parts[5]),
		Body:        strings.TrimSpace(parts[6]),
	}, nil
}

// IsAncestor reports whether ancestor is a reachable ancestor of descendant.
func (r *Repository) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	_, err := r.runGit(ctx, "merge-base", "--is-ancestor", ancestor, descendant)
	if err != nil {
		// Exit code 1 means "not an ancestor" — not a hard error.
		if isExitCode(err, 1) {
			return false, nil
		}
		return false, fmt.Errorf("is-ancestor %q %q: %w", ancestor, descendant, err)
	}
	return true, nil
}

// CherryPick applies the changes introduced by hash onto HEAD.
func (r *Repository) CherryPick(ctx context.Context, hash string) error {
	if _, err := r.runGit(ctx, "cherry-pick", hash); err != nil {
		return fmt.Errorf("cherry-pick %q: %w", hash, err)
	}
	return nil
}

// AbortCherryPick aborts an in-progress cherry-pick and restores the original
// HEAD state.
func (r *Repository) AbortCherryPick(ctx context.Context) error {
	if _, err := r.runGit(ctx, "cherry-pick", "--abort"); err != nil {
		return fmt.Errorf("cherry-pick --abort: %w", err)
	}
	return nil
}

// CreateBranch creates a new branch at HEAD and checks it out.
func (r *Repository) CreateBranch(ctx context.Context, name string) error {
	if _, err := r.runGit(ctx, "checkout", "-b", name); err != nil {
		return fmt.Errorf("create branch %q: %w", name, err)
	}
	return nil
}

// GetHead returns the full commit hash that HEAD currently points to.
func (r *Repository) GetHead(ctx context.Context) (string, error) {
	out, err := r.runGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get HEAD: %w", err)
	}
	return out, nil
}

// isExitCode reports whether err wraps an *exec.ExitError with the given code.
func isExitCode(err error, code int) bool {
	type exitCoder interface {
		ExitCode() int
	}
	var ec exitCoder
	// Walk the error chain manually to avoid importing errors package cycle.
	for e := err; e != nil; {
		if coder, ok := e.(exitCoder); ok {
			ec = coder
			break
		}
		// Unwrap one level.
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
		} else {
			break
		}
	}
	return ec != nil && ec.ExitCode() == code
}
