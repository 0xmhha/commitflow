package validate

import "testing"

func TestEscapeLikePattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no special chars", input: "abc123", want: "abc123"},
		{name: "percent", input: "abc%123", want: `abc\%123`},
		{name: "underscore", input: "abc_123", want: `abc\_123`},
		{name: "backslash", input: `abc\123`, want: `abc\\123`},
		{name: "all special", input: `%_\`, want: `\%\_\\`},
		{name: "empty string", input: "", want: ""},
		{name: "no escaping needed hex", input: "deadbeef", want: "deadbeef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapeLikePattern(tt.input)
			if got != tt.want {
				t.Errorf("EscapeLikePattern(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "valid lowercase", input: "abcdef", want: true},
		{name: "valid uppercase", input: "ABCDEF", want: true},
		{name: "valid mixed", input: "aBcDeF", want: true},
		{name: "valid digits", input: "0123456789", want: true},
		{name: "valid hex hash", input: "abc1234", want: true},
		{name: "empty string", input: "", want: false},
		{name: "contains g", input: "abcdefg", want: false},
		{name: "contains space", input: "abc 123", want: false},
		{name: "contains percent", input: "abc%", want: false},
		{name: "contains underscore", input: "abc_", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHexString(tt.input)
			if got != tt.want {
				t.Errorf("IsHexString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeStderr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no sensitive data",
			input: "fatal: not a git repository",
			want:  "fatal: not a git repository",
		},
		{
			name:  "truncate long stderr",
			input: string(make([]byte, 300)),
			want:  string(make([]byte, 256)) + "...(truncated)",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeStderr(tt.input)
			if tt.name == "truncate long stderr" {
				if len(got) > 280 {
					t.Errorf("SanitizeStderr() output too long: %d chars", len(got))
				}
				return
			}
			if got != tt.want {
				t.Errorf("SanitizeStderr(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
