package analysis

import "testing"

func TestCategoryLabel(t *testing.T) {
	tests := []struct {
		category, want string
	}{
		{"bug_fix", "[FIX]"},
		{"feature", "[FEAT]"},
		{"refactor", "[REFACTOR]"},
		{"security", "[SEC]"},
		{"performance", "[PERF]"},
		{"docs", "[DOCS]"},
		{"chore", "[CHORE]"},
		{"breaking_change", "[BREAK]"},
		{"unknown_category", "[CHORE]"},
		{"", "[CHORE]"},
	}

	for _, tt := range tests {
		got := CategoryLabel(tt.category)
		if got != tt.want {
			t.Errorf("CategoryLabel(%q) = %q, want %q", tt.category, got, tt.want)
		}
	}
}

func TestImpactLevel(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{0, "low"},
		{1, "low"},
		{3, "low"},
		{4, "medium"},
		{6, "medium"},
		{7, "high"},
		{8, "high"},
		{9, "critical"},
		{10, "critical"},
		{11, "critical"},
	}

	for _, tt := range tests {
		got := ImpactLevel(tt.score)
		if got != tt.want {
			t.Errorf("ImpactLevel(%d) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestIsSecurityRelevant(t *testing.T) {
	tests := []struct {
		category, message string
		want              bool
	}{
		{"security", "anything", true},
		{"bug_fix", "Fix security vulnerability in auth", true},
		{"feature", "Add new XSS protection", true},
		{"chore", "Update documentation", false},
		{"feature", "Add new button", false},
		{"bug_fix", "Fix SQL injection in user input", true},
		{"refactor", "Refactor authentication module", true},
		{"chore", "Update crypto library version", true},
		{"docs", "Document TLS configuration", true},
		{"performance", "Optimize string handling", false},
	}

	for _, tt := range tests {
		got := IsSecurityRelevant(tt.category, tt.message)
		if got != tt.want {
			t.Errorf("IsSecurityRelevant(%q, %q) = %v, want %v",
				tt.category, tt.message, got, tt.want)
		}
	}
}
