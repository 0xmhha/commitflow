package git

import (
	"testing"
)

func TestParseCommit_Valid(t *testing.T) {
	// 7 NUL-separated fields: hash, parents, author, email, date, subject, body
	raw := "abc123def456\x00parent123\x00John Doe\x00john@example.com\x002025-01-15T10:30:00+09:00\x00Fix memory leak\x00Detailed body text"

	c, err := parseCommit(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Hash != "abc123def456" {
		t.Errorf("Hash = %q, want %q", c.Hash, "abc123def456")
	}
	if c.ParentHash != "parent123" {
		t.Errorf("ParentHash = %q, want %q", c.ParentHash, "parent123")
	}
	if c.Author != "John Doe" {
		t.Errorf("Author = %q, want %q", c.Author, "John Doe")
	}
	if c.AuthorEmail != "john@example.com" {
		t.Errorf("AuthorEmail = %q, want %q", c.AuthorEmail, "john@example.com")
	}
	if c.Date != "2025-01-15T10:30:00+09:00" {
		t.Errorf("Date = %q, want %q", c.Date, "2025-01-15T10:30:00+09:00")
	}
	if c.Subject != "Fix memory leak" {
		t.Errorf("Subject = %q, want %q", c.Subject, "Fix memory leak")
	}
	if c.Body != "Detailed body text" {
		t.Errorf("Body = %q, want %q", c.Body, "Detailed body text")
	}
}

func TestParseCommit_MergeCommitMultipleParents(t *testing.T) {
	raw := "abc123\x00parent1 parent2 parent3\x00Author\x00a@b.com\x002025-01-01\x00Merge branch\x00"

	c, err := parseCommit(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the first parent should be stored.
	if c.ParentHash != "parent1" {
		t.Errorf("ParentHash = %q, want %q (first parent only)", c.ParentHash, "parent1")
	}
}

func TestParseCommit_RootCommitNoParent(t *testing.T) {
	raw := "abc123\x00\x00Author\x00a@b.com\x002025-01-01\x00Initial commit\x00"

	c, err := parseCommit(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.ParentHash != "" {
		t.Errorf("ParentHash = %q, want empty string for root commit", c.ParentHash)
	}
}

func TestParseCommit_InsufficientFields(t *testing.T) {
	raw := "abc123\x00parent\x00Author"

	_, err := parseCommit(raw)
	if err == nil {
		t.Fatal("expected error for insufficient fields, got nil")
	}
}

func TestIsExitCode(t *testing.T) {
	// nil error should return false.
	if isExitCode(nil, 1) {
		t.Error("isExitCode(nil, 1) = true, want false")
	}
}
