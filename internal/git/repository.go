package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Repository is a handle to a local git repository.
type Repository struct {
	path string
}

// NewRepository validates that path contains a git repository and returns
// a Repository handle for it.
func NewRepository(path string) (*Repository, error) {
	r := &Repository{path: path}

	// Validate the directory exists.
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("repository path %q: %w", path, err)
	}

	ctx := context.Background()
	if _, err := r.runGit(ctx, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("not a git repository %q: %w", path, err)
	}

	return r, nil
}

// Path returns the filesystem path of the repository.
func (r *Repository) Path() string {
	return r.path
}

// ResolveHash resolves any ref (branch name, tag, short hash, symbolic ref)
// to its full 40-character commit hash.
func (r *Repository) ResolveHash(ctx context.Context, ref string) (string, error) {
	out, err := r.runGit(ctx, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("resolve hash %q: %w", ref, err)
	}
	return out, nil
}

// MergeBase returns the best common ancestor of ref1 and ref2.
func (r *Repository) MergeBase(ctx context.Context, ref1, ref2 string) (string, error) {
	out, err := r.runGit(ctx, "merge-base", ref1, ref2)
	if err != nil {
		return "", fmt.Errorf("merge-base %q %q: %w", ref1, ref2, err)
	}
	return out, nil
}

// AddRemote adds a named remote. If the remote already exists the error is
// silently ignored so callers can treat this as idempotent.
func (r *Repository) AddRemote(ctx context.Context, name, url string) error {
	_, err := r.runGit(ctx, "remote", "add", name, url)
	if err != nil {
		// "already exists" is not a failure for our purposes.
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("add remote %q %q: %w", name, url, err)
	}
	return nil
}

// Fetch fetches all objects from the named remote.
func (r *Repository) Fetch(ctx context.Context, remote string) error {
	if _, err := r.runGit(ctx, "fetch", remote); err != nil {
		return fmt.Errorf("fetch %q: %w", remote, err)
	}
	return nil
}

// FileExists reports whether path exists in the working tree.
func (r *Repository) FileExists(ctx context.Context, path string) bool {
	_, err := r.runGit(ctx, "ls-files", "--error-unmatch", path)
	return err == nil
}

// runGit executes git with the given arguments inside r.path, returning
// trimmed stdout. The error wraps the command and stderr output when git exits
// non-zero, and supports context cancellation via exec.CommandContext.
func (r *Repository) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.path

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderrStr)
	}

	return strings.TrimSpace(stdout.String()), nil
}
