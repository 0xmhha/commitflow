package validate

import "strings"

const maxStderrLen = 256

// EscapeLikePattern escapes SQL LIKE wildcard characters (%, _, \) in a
// pattern string so they are treated as literals. Use with ESCAPE '\'.
func EscapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// IsHexString reports whether s is a non-empty string of hexadecimal digits.
func IsHexString(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if !isHexDigit(c) {
			return false
		}
	}
	return true
}

// SanitizeStderr truncates stderr output to a safe length to prevent
// information leakage in error messages.
func SanitizeStderr(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) > maxStderrLen {
		return s[:maxStderrLen] + "...(truncated)"
	}
	return s
}
