package analysis

import (
	"testing"

	"github.com/0xmhha/commitflow/internal/git"
)

func TestCollectFilePaths(t *testing.T) {
	diffs := []git.FileDiff{
		{Path: "main.go"},
		{Path: "cmd/root.go"},
		{Path: "internal/ai/client.go"},
	}

	paths := collectFilePaths(diffs)
	if len(paths) != 3 {
		t.Fatalf("collectFilePaths() returned %d paths, want 3", len(paths))
	}

	expected := []string{"main.go", "cmd/root.go", "internal/ai/client.go"}
	for i, want := range expected {
		if paths[i] != want {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], want)
		}
	}
}

func TestCollectFilePaths_Empty(t *testing.T) {
	paths := collectFilePaths(nil)
	if len(paths) != 0 {
		t.Errorf("collectFilePaths(nil) returned %d paths, want 0", len(paths))
	}
}

func TestTruncateMessage(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a long message", 10, "this is a ..."},
		{"", 5, ""},
		{"hello", 5, "hello"},
	}

	for _, tt := range tests {
		got := truncateMessage(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateMessage(%q, %d) = %q, want %q",
				tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestMinLen(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{5, 10, 5},
		{10, 5, 5},
		{5, 5, 5},
		{0, 5, 0},
	}

	for _, tt := range tests {
		got := minLen(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("minLen(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
