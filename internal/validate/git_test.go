package validate

import "testing"

func TestRef(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		// Valid refs
		{name: "valid branch", ref: "main", wantErr: false},
		{name: "valid branch with slash", ref: "feature/login", wantErr: false},
		{name: "valid tag", ref: "v1.0.0", wantErr: false},
		{name: "valid HEAD", ref: "HEAD", wantErr: false},
		{name: "valid short hash", ref: "abc1234", wantErr: false},
		{name: "valid full hash", ref: "abc1234567890abcdef1234567890abcdef123456", wantErr: false},
		{name: "valid ref with tilde", ref: "HEAD~3", wantErr: false},
		{name: "valid ref with caret", ref: "HEAD^2", wantErr: false},
		{name: "valid ref with at", ref: "HEAD@{0}", wantErr: false},
		{name: "valid hyphenated branch", ref: "fix-bug-123", wantErr: false},
		{name: "valid dotted branch", ref: "release.1.0", wantErr: false},
		{name: "valid underscore branch", ref: "my_branch", wantErr: false},
		{name: "valid origin ref", ref: "origin/main", wantErr: false},

		// Invalid refs
		{name: "empty ref", ref: "", wantErr: true},
		{name: "leading dash", ref: "-n1", wantErr: true},
		{name: "double dash flag", ref: "--exec=cmd", wantErr: true},
		{name: "upload-pack injection", ref: "--upload-pack=evil", wantErr: true},
		{name: "config injection", ref: "-c core.sshCommand=evil", wantErr: true},
		{name: "null byte", ref: "main\x00injected", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Ref(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("Ref(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			}
		})
	}
}

func TestHash(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		// Valid hashes
		{name: "7-char short hash", hash: "abc1234", wantErr: false},
		{name: "8-char short hash", hash: "abc12345", wantErr: false},
		{name: "39-char hash", hash: "abc1234567890abcdef1234567890abcdef1234", wantErr: false},
		{name: "full 40-char hash", hash: "abc1234567890abcdef1234567890abcdef12345", wantErr: false},
		{name: "uppercase hex", hash: "ABCDEF1", wantErr: false},
		{name: "mixed case hex", hash: "aBcDeF1", wantErr: false},

		// Invalid hashes
		{name: "empty hash", hash: "", wantErr: true},
		{name: "too short (6 chars)", hash: "abc123", wantErr: true},
		{name: "too long (41 chars)", hash: "abc1234567890abcdef1234567890abcdef1234567", wantErr: true},
		{name: "non-hex character g", hash: "abc123g", wantErr: true},
		{name: "non-hex character z", hash: "zzzzzzz", wantErr: true},
		{name: "contains percent", hash: "abc123%", wantErr: true},
		{name: "contains underscore", hash: "abc_123", wantErr: true},
		{name: "leading dash", hash: "-abcdef1", wantErr: true},
		{name: "spaces", hash: "abc 123", wantErr: true},
		{name: "null byte", hash: "abc\x00123", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Hash(tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("Hash(%q) error = %v, wantErr %v", tt.hash, err, tt.wantErr)
			}
		})
	}
}

func TestRemoteName(t *testing.T) {
	tests := []struct {
		name    string
		remote  string
		wantErr bool
	}{
		// Valid
		{name: "origin", remote: "origin", wantErr: false},
		{name: "upstream", remote: "upstream", wantErr: false},
		{name: "with hyphen", remote: "my-remote", wantErr: false},
		{name: "with underscore", remote: "my_remote", wantErr: false},

		// Invalid
		{name: "empty", remote: "", wantErr: true},
		{name: "leading dash", remote: "-evil", wantErr: true},
		{name: "double dash", remote: "--upload-pack", wantErr: true},
		{name: "null byte", remote: "origin\x00evil", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RemoteName(tt.remote)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoteName(%q) error = %v, wantErr %v", tt.remote, err, tt.wantErr)
			}
		})
	}
}
