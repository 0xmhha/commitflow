package sync

import (
	"testing"
)

func TestBranchNameForHash(t *testing.T) {
	tests := []struct {
		hash, want string
	}{
		{"abc123def456789012345678901234567890abcd", "upstream/abc123de"},
		{"short", "upstream/short"},
		{"12345678", "upstream/12345678"},
		{"123456789", "upstream/12345678"},
	}

	for _, tt := range tests {
		got := branchNameForHash(tt.hash)
		if got != tt.want {
			t.Errorf("branchNameForHash(%q) = %q, want %q", tt.hash, got, tt.want)
		}
	}
}
