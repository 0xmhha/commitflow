package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// Sentinel errors for AI client operations.
var (
	ErrBudgetExhausted = errors.New("AI budget limit reached")
	ErrClaudeNotFound  = errors.New("claude CLI not found in PATH")
	ErrClaudeAuth      = errors.New("claude CLI authentication expired")
)

// Client wraps the Claude Code CLI for structured AI analysis.
type Client struct {
	model             string
	maxBudgetPerCall  float64
	totalBudgetLimit  float64
	delayBetweenCalls time.Duration
	maxRetries        int
	retryBackoff      time.Duration
	dryRun            bool
	verbose           bool

	accumulatedCost float64
	mu              sync.Mutex
}

// ClientConfig holds configuration for the AI client.
type ClientConfig struct {
	Model             string
	MaxBudgetPerCall  float64
	TotalBudgetLimit  float64
	DelayBetweenCalls time.Duration
	MaxRetries        int
	RetryBackoff      time.Duration
	DryRun            bool
	Verbose           bool
}

// NewClient constructs a Client from the provided configuration.
func NewClient(cfg ClientConfig) *Client {
	return &Client{
		model:             cfg.Model,
		maxBudgetPerCall:  cfg.MaxBudgetPerCall,
		totalBudgetLimit:  cfg.TotalBudgetLimit,
		delayBetweenCalls: cfg.DelayBetweenCalls,
		maxRetries:        cfg.MaxRetries,
		retryBackoff:      cfg.RetryBackoff,
		dryRun:            cfg.DryRun,
		verbose:           cfg.Verbose,
	}
}

// AccumulatedCost returns the total cost accumulated across all calls so far.
// It is safe to call concurrently.
func (c *Client) AccumulatedCost() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accumulatedCost
}

// Call executes a Claude CLI command with the given prompt and JSON schema.
// It applies budget checks, dry-run short-circuits, retry with exponential
// backoff, and rate-limiting delay before returning.
func (c *Client) Call(ctx context.Context, prompt string, jsonSchema string) (*Response, error) {
	if err := c.checkBudget(); err != nil {
		return nil, err
	}

	if c.dryRun {
		slog.Info("dry-run: skipping Claude call", "prompt_len", len(prompt))
		return nil, nil
	}

	resp, err := c.callWithRetry(ctx, prompt, jsonSchema)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.accumulatedCost += resp.TotalCostUSD
	c.mu.Unlock()

	if c.delayBetweenCalls > 0 {
		select {
		case <-time.After(c.delayBetweenCalls):
		case <-ctx.Done():
			return resp, ctx.Err()
		}
	}

	return resp, nil
}

// CheckClaude verifies the claude CLI binary is available in PATH.
func CheckClaude() error {
	if _, err := exec.LookPath("claude"); err != nil {
		return ErrClaudeNotFound
	}
	return nil
}

// checkBudget returns ErrBudgetExhausted when the total budget limit is set
// and the accumulated cost has reached or exceeded it.
func (c *Client) checkBudget() error {
	if c.totalBudgetLimit <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accumulatedCost >= c.totalBudgetLimit {
		return fmt.Errorf("%w: accumulated $%.4f, limit $%.4f",
			ErrBudgetExhausted, c.accumulatedCost, c.totalBudgetLimit)
	}
	return nil
}

// callWithRetry executes the claude CLI call up to maxRetries times using
// exponential backoff, returning the parsed Response on success.
func (c *Client) callWithRetry(ctx context.Context, prompt, jsonSchema string) (*Response, error) {
	var lastErr error

	maxAttempts := c.maxRetries
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := c.retryBackoff * (1 << (attempt - 1))
			slog.Info("retrying Claude call", "attempt", attempt+1, "backoff", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := c.execClaude(ctx, prompt, jsonSchema)
		if err == nil {
			return resp, nil
		}

		// Authentication errors are fatal - do not retry.
		if errors.Is(err, ErrClaudeAuth) {
			return nil, err
		}

		lastErr = err
		slog.Warn("Claude call failed", "attempt", attempt+1, "error", err)
	}

	return nil, fmt.Errorf("claude call failed after %d attempts: %w", maxAttempts, lastErr)
}

// execClaude builds and runs the claude CLI command, returning the parsed response.
func (c *Client) execClaude(ctx context.Context, prompt, jsonSchema string) (*Response, error) {
	args := c.buildArgs(jsonSchema)

	if c.verbose {
		slog.Debug("executing claude", "args", args, "prompt_len", len(prompt))
	}

	cmd := exec.CommandContext(ctx, "claude", args...)

	// Pass the prompt via stdin to avoid command-line length limits.
	cmd.Stdin = bytes.NewBufferString(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if isAuthError(stderrStr) {
			return nil, ErrClaudeAuth
		}
		return nil, fmt.Errorf("claude exec: %w: %s", err, stderrStr)
	}

	if c.verbose {
		slog.Debug("claude response received", "bytes", stdout.Len())
	}

	return ParseResponse(stdout.Bytes())
}

// buildArgs constructs the claude CLI argument slice.
func (c *Client) buildArgs(jsonSchema string) []string {
	args := []string{
		"-p", "-", // read prompt from stdin
		"--output-format", "json",
		"--model", c.model,
		"--tools", "",
		"--no-session-persistence",
	}

	if jsonSchema != "" {
		args = append(args, "--json-schema", jsonSchema)
	}

	if c.maxBudgetPerCall > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.4f", c.maxBudgetPerCall))
	}

	return args
}

// isAuthError detects authentication-related errors in claude CLI stderr output.
func isAuthError(stderr string) bool {
	authPhrases := []string{
		"authentication",
		"not logged in",
		"unauthorized",
		"token expired",
		"invalid api key",
	}
	lower := toLower(stderr)
	for _, phrase := range authPhrases {
		if contains(lower, phrase) {
			return true
		}
	}
	return false
}

// toLower is a simple ASCII-only lowercase helper to avoid importing strings.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// contains reports whether substr is within s.
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
