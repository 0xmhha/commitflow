package validate

import (
	"fmt"
	"strings"
)

// Ref validates that a git ref string is safe to pass to git commands.
// It rejects empty refs, refs starting with '-' (flag injection), and
// refs containing NUL bytes.
func Ref(ref string) error {
	if ref == "" {
		return fmt.Errorf("ref must not be empty")
	}
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("ref must not start with '-': %q", ref)
	}
	if strings.ContainsRune(ref, '\x00') {
		return fmt.Errorf("ref must not contain null bytes")
	}
	return nil
}

// Hash validates that a string is a valid hex commit hash (7-40 characters,
// hex digits only). This is stricter than Ref and should be used when the
// value is expected to be a commit hash rather than a symbolic ref.
func Hash(hash string) error {
	if len(hash) < 7 || len(hash) > 40 {
		return fmt.Errorf("hash must be 7-40 hex characters, got %d", len(hash))
	}
	for _, c := range hash {
		if !isHexDigit(c) {
			return fmt.Errorf("hash contains non-hex character: %q", string(c))
		}
	}
	return nil
}

// RemoteName validates that a git remote name is safe.
// It rejects empty names, names starting with '-', and names containing
// NUL bytes.
func RemoteName(name string) error {
	if name == "" {
		return fmt.Errorf("remote name must not be empty")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("remote name must not start with '-': %q", name)
	}
	if strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("remote name must not contain null bytes")
	}
	return nil
}

func isHexDigit(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
