package git

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xmhha/commitflow/internal/validate"
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
	cleanPath := filepath.Clean(path)
	if _, err := os.Stat(cleanPath); err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}
	r.path = cleanPath

	ctx := context.Background()
	if _, err := r.runGit(ctx, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
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
	if err := validate.Ref(ref); err != nil {
		return "", fmt.Errorf("resolve hash: %w", err)
	}
	out, err := r.runGit(ctx, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("resolve hash %q: %w", ref, err)
	}
	return out, nil
}

// MergeBase returns the best common ancestor of ref1 and ref2.
func (r *Repository) MergeBase(ctx context.Context, ref1, ref2 string) (string, error) {
	if err := validate.Ref(ref1); err != nil {
		return "", fmt.Errorf("merge-base ref1: %w", err)
	}
	if err := validate.Ref(ref2); err != nil {
		return "", fmt.Errorf("merge-base ref2: %w", err)
	}
	out, err := r.runGit(ctx, "merge-base", ref1, ref2)
	if err != nil {
		return "", fmt.Errorf("merge-base %q %q: %w", ref1, ref2, err)
	}
	return out, nil
}

// AddRemote adds a named remote. If the remote already exists the error is
// silently ignored so callers can treat this as idempotent.
func (r *Repository) AddRemote(ctx context.Context, name, url string) error {
	if err := validate.RemoteName(name); err != nil {
		return fmt.Errorf("add remote: %w", err)
	}
	_, err := r.runGit(ctx, "remote", "add", name, url)
	if err != nil {
		// "already exists" is not a failure for our purposes.
		var gitErr *GitError
		if asGitError(err, &gitErr) && strings.Contains(gitErr.Stderr, "already exists") {
			return nil
		}
		return fmt.Errorf("add remote %q: %w", name, err)
	}
	return nil
}

// Fetch fetches all objects from the named remote.
func (r *Repository) Fetch(ctx context.Context, remote string) error {
	if err := validate.RemoteName(remote); err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
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

// GitError wraps a git command failure with stderr detail accessible for
// programmatic checks (e.g. "already exists") but not exposed in Error().
type GitError struct {
	Subcommand string
	Err        error
	Stderr     string
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git %s: %s", e.Subcommand, e.Err)
}

func (e *GitError) Unwrap() error { return e.Err }

// asGitError is a helper to extract a *GitError from an error chain.
func asGitError(err error, target **GitError) bool {
	type unwrapper interface{ Unwrap() error }
	for e := err; e != nil; {
		if ge, ok := e.(*GitError); ok {
			*target = ge
			return true
		}
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}

const defaultGitTimeout = 5 * time.Minute

// runGit executes git with the given arguments inside r.path, returning
// trimmed stdout. On failure it returns a *GitError with stderr accessible
// programmatically but not included in the user-facing error string.
func (r *Repository) runGit(ctx context.Context, args ...string) (string, error) {
	// Apply a default timeout if the context has no deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultGitTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.path

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		slog.Debug("git command failed",
			"subcommand", args[0],
			"stderr", stderrStr,
		)
		subcmd := ""
		if len(args) > 0 {
			subcmd = args[0]
		}
		return "", &GitError{
			Subcommand: subcmd,
			Err:        err,
			Stderr:     stderrStr,
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}
