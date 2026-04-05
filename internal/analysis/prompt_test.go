package analysis

import (
	"strings"
	"testing"
)

func TestBuildAnalysisPrompt_BasicFields(t *testing.T) {
	data := PromptData{
		Hash:         "abc1234567890",
		Author:       "Test Author",
		AuthorEmail:  "test@example.com",
		Date:         "2025-03-15",
		Message:      "Fix critical bug in parser",
		FilesChanged: []string{"main.go", "parser.go"},
		Additions:    50,
		Deletions:    20,
		Diff:         "diff content here",
	}

	prompt, err := BuildAnalysisPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key content is present.
	checks := []string{
		"abc1234567890",
		"Test Author",
		"test@example.com",
		"2025-03-15",
		"Fix critical bug in parser",
		"Files changed: 2",
		"Additions: 50",
		"Deletions: 20",
		"diff content here",
	}

	for _, want := range checks {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestBuildAnalysisPrompt_Truncated(t *testing.T) {
	data := PromptData{
		Hash:           "abc123",
		Author:         "Author",
		AuthorEmail:    "a@b.com",
		Date:           "2025-01-01",
		Message:        "Large commit",
		FilesChanged:   []string{"a.go"},
		DiffTruncated:  true,
		TruncationNote: "Diff too large (30000 lines)",
		StatSummary:    "10 files changed, 500 insertions(+), 300 deletions(-)",
	}

	prompt, err := BuildAnalysisPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "Diff too large (30000 lines)") {
		t.Error("prompt should contain truncation note")
	}
	if !strings.Contains(prompt, "10 files changed") {
		t.Error("prompt should contain stat summary")
	}
}

func TestBuildAnalysisPrompt_NoDiff(t *testing.T) {
	data := PromptData{
		Hash:        "abc123",
		Author:      "Author",
		AuthorEmail: "a@b.com",
		Date:        "2025-01-01",
		Message:     "Merge commit",
	}

	prompt, err := BuildAnalysisPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still contain the analysis instructions.
	if !strings.Contains(prompt, "Analyze this commit") {
		t.Error("prompt missing analysis instructions")
	}
}
