package ai

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	cfg := ClientConfig{
		Model:             "opus",
		MaxBudgetPerCall:  1.0,
		TotalBudgetLimit:  10.0,
		DelayBetweenCalls: 2 * time.Second,
		MaxRetries:        5,
		RetryBackoff:      3 * time.Second,
		DryRun:            true,
		Verbose:           true,
	}

	c := NewClient(cfg)
	if c.model != "opus" {
		t.Errorf("model = %q, want %q", c.model, "opus")
	}
	if c.maxBudgetPerCall != 1.0 {
		t.Errorf("maxBudgetPerCall = %f, want 1.0", c.maxBudgetPerCall)
	}
	if c.totalBudgetLimit != 10.0 {
		t.Errorf("totalBudgetLimit = %f, want 10.0", c.totalBudgetLimit)
	}
	if c.delayBetweenCalls != 2*time.Second {
		t.Errorf("delayBetweenCalls = %v, want 2s", c.delayBetweenCalls)
	}
	if c.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", c.maxRetries)
	}
	if !c.dryRun {
		t.Error("dryRun = false, want true")
	}
	if !c.verbose {
		t.Error("verbose = false, want true")
	}
}

func TestAccumulatedCost_InitiallyZero(t *testing.T) {
	c := NewClient(ClientConfig{})
	if got := c.AccumulatedCost(); got != 0 {
		t.Errorf("AccumulatedCost() = %f, want 0", got)
	}
}

func TestCheckBudget_NoBudgetLimit(t *testing.T) {
	c := NewClient(ClientConfig{TotalBudgetLimit: 0})
	if err := c.checkBudget(); err != nil {
		t.Errorf("checkBudget() with no limit returned error: %v", err)
	}
}

func TestCheckBudget_UnderLimit(t *testing.T) {
	c := NewClient(ClientConfig{TotalBudgetLimit: 10.0})
	c.accumulatedCost = 5.0
	if err := c.checkBudget(); err != nil {
		t.Errorf("checkBudget() under limit returned error: %v", err)
	}
}

func TestCheckBudget_AtLimit(t *testing.T) {
	c := NewClient(ClientConfig{TotalBudgetLimit: 10.0})
	c.accumulatedCost = 10.0
	err := c.checkBudget()
	if err == nil {
		t.Fatal("checkBudget() at limit should return error")
	}
}

func TestCheckBudget_OverLimit(t *testing.T) {
	c := NewClient(ClientConfig{TotalBudgetLimit: 10.0})
	c.accumulatedCost = 15.0
	err := c.checkBudget()
	if err == nil {
		t.Fatal("checkBudget() over limit should return error")
	}
}

func TestBuildArgs_Basic(t *testing.T) {
	c := NewClient(ClientConfig{Model: "sonnet"})
	args := c.buildArgs("")

	expected := []string{
		"-p", "-",
		"--output-format", "json",
		"--model", "sonnet",
		"--tools", "",
		"--no-session-persistence",
	}

	if len(args) != len(expected) {
		t.Fatalf("buildArgs() returned %d args, want %d", len(args), len(expected))
	}

	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestBuildArgs_WithSchema(t *testing.T) {
	c := NewClient(ClientConfig{Model: "sonnet"})
	args := c.buildArgs(`{"type":"object"}`)

	found := false
	for i, arg := range args {
		if arg == "--json-schema" && i+1 < len(args) {
			found = true
			if args[i+1] != `{"type":"object"}` {
				t.Errorf("json-schema value = %q, want %q", args[i+1], `{"type":"object"}`)
			}
		}
	}
	if !found {
		t.Error("--json-schema flag not found in args")
	}
}

func TestBuildArgs_WithBudget(t *testing.T) {
	c := NewClient(ClientConfig{Model: "sonnet", MaxBudgetPerCall: 0.5})
	args := c.buildArgs("")

	found := false
	for i, arg := range args {
		if arg == "--max-budget-usd" && i+1 < len(args) {
			found = true
			if args[i+1] != "0.5000" {
				t.Errorf("budget value = %q, want %q", args[i+1], "0.5000")
			}
		}
	}
	if !found {
		t.Error("--max-budget-usd flag not found in args")
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		stderr string
		want   bool
	}{
		{"Error: authentication failed", true},
		{"Error: not logged in to Claude", true},
		{"Error: unauthorized access", true},
		{"Error: token expired", true},
		{"Error: invalid api key", true},
		{"Error: rate limit exceeded", false},
		{"Success", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isAuthError(tt.stderr)
		if got != tt.want {
			t.Errorf("isAuthError(%q) = %v, want %v", tt.stderr, got, tt.want)
		}
	}
}

func TestToLower(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"HELLO", "hello"},
		{"Hello World", "hello world"},
		{"already lower", "already lower"},
		{"123ABC", "123abc"},
		{"", ""},
	}

	for _, tt := range tests {
		got := toLower(tt.input)
		if got != tt.want {
			t.Errorf("toLower(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s, substr string
		want      bool
	}{
		{"hello world", "world", true},
		{"hello world", "xyz", false},
		{"hello", "", true},
		{"", "hello", false},
		{"abc", "abcd", false},
	}

	for _, tt := range tests {
		got := contains(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestValidCategories(t *testing.T) {
	cats := ValidCategories()
	if len(cats) != 8 {
		t.Errorf("ValidCategories() returned %d items, want 8", len(cats))
	}
}

func TestValidStatuses(t *testing.T) {
	statuses := ValidStatuses()
	if len(statuses) != 4 {
		t.Errorf("ValidStatuses() returned %d items, want 4", len(statuses))
	}
}

func TestValidConflictLikelihoods(t *testing.T) {
	likelihoods := ValidConflictLikelihoods()
	if len(likelihoods) != 4 {
		t.Errorf("ValidConflictLikelihoods() returned %d items, want 4", len(likelihoods))
	}
}
