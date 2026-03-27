package analysis

import "strings"

// securityKeywords is the set of lowercase terms that flag security relevance.
var securityKeywords = []string{
	"security", "vulnerability", "exploit", "cve", "xss", "sql injection",
	"injection", "overflow", "sanitize", "authentication", "authorization",
	"auth", "privilege", "permission", "crypto", "encrypt", "decrypt",
	"hash", "token", "secret", "credential", "password", "tls", "ssl",
	"certificate", "signature", "bypass", "attack",
}

// IsSecurityRelevant returns true when the category is "security" or when
// the commit message contains known security-related keywords.
func IsSecurityRelevant(category string, message string) bool {
	if category == "security" {
		return true
	}
	lower := strings.ToLower(message)
	for _, kw := range securityKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// ImpactLevel converts a numeric impact score (1-10) to a human-readable tier.
// Scores outside the 1-10 range are treated as boundary values.
func ImpactLevel(score int) string {
	switch {
	case score <= 3:
		return "low"
	case score <= 6:
		return "medium"
	case score <= 8:
		return "high"
	default:
		return "critical"
	}
}

// CategoryLabel returns a short bracketed label for a commit category.
// Unknown categories fall back to "[CHORE]".
func CategoryLabel(category string) string {
	labels := map[string]string{
		"bug_fix":         "[FIX]",
		"feature":         "[FEAT]",
		"refactor":        "[REFACTOR]",
		"security":        "[SEC]",
		"performance":     "[PERF]",
		"docs":            "[DOCS]",
		"chore":           "[CHORE]",
		"breaking_change": "[BREAK]",
	}
	if label, ok := labels[category]; ok {
		return label
	}
	return "[CHORE]"
}
