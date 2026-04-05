package git

import (
	"testing"
)

func TestParseNumstat_Normal(t *testing.T) {
	output := "10\t5\tsrc/main.go\n3\t1\tREADME.md"

	files := parseNumstat(output)
	if len(files) != 2 {
		t.Fatalf("parseNumstat() returned %d files, want 2", len(files))
	}

	if files[0].Path != "src/main.go" {
		t.Errorf("files[0].Path = %q, want %q", files[0].Path, "src/main.go")
	}
	if files[0].Additions != 10 {
		t.Errorf("files[0].Additions = %d, want 10", files[0].Additions)
	}
	if files[0].Deletions != 5 {
		t.Errorf("files[0].Deletions = %d, want 5", files[0].Deletions)
	}
	if files[0].Binary {
		t.Error("files[0].Binary = true, want false")
	}

	if files[1].Path != "README.md" {
		t.Errorf("files[1].Path = %q, want %q", files[1].Path, "README.md")
	}
}

func TestParseNumstat_BinaryFile(t *testing.T) {
	output := "-\t-\timages/logo.png"

	files := parseNumstat(output)
	if len(files) != 1 {
		t.Fatalf("parseNumstat() returned %d files, want 1", len(files))
	}

	if !files[0].Binary {
		t.Error("Binary = false, want true")
	}
	if files[0].Path != "images/logo.png" {
		t.Errorf("Path = %q, want %q", files[0].Path, "images/logo.png")
	}
}

func TestParseNumstat_Empty(t *testing.T) {
	files := parseNumstat("")
	if len(files) != 0 {
		t.Errorf("parseNumstat(\"\") returned %d files, want 0", len(files))
	}
}

func TestParseNumstat_WithEmptyLines(t *testing.T) {
	output := "5\t2\tfile.go\n\n\n3\t1\tother.go\n"

	files := parseNumstat(output)
	if len(files) != 2 {
		t.Fatalf("parseNumstat() returned %d files, want 2 (skipping empty lines)", len(files))
	}
}

func TestParseNumstatLine_Rename(t *testing.T) {
	line := "5\t2\told.go => new.go"

	fd := parseNumstatLine(line)
	if fd == nil {
		t.Fatal("parseNumstatLine() returned nil")
	}
	if fd.Path != "new.go" {
		t.Errorf("Path = %q, want %q", fd.Path, "new.go")
	}
	if fd.OldPath != "old.go" {
		t.Errorf("OldPath = %q, want %q", fd.OldPath, "old.go")
	}
}

func TestSplitRenamePath_Simple(t *testing.T) {
	oldPath, newPath := splitRenamePath("old.go => new.go")
	if oldPath != "old.go" {
		t.Errorf("oldPath = %q, want %q", oldPath, "old.go")
	}
	if newPath != "new.go" {
		t.Errorf("newPath = %q, want %q", newPath, "new.go")
	}
}

func TestSplitRenamePath_BraceStyle(t *testing.T) {
	oldPath, newPath := splitRenamePath("src/{old => new}/file.go")
	if oldPath != "src/old/file.go" {
		t.Errorf("oldPath = %q, want %q", oldPath, "src/old/file.go")
	}
	if newPath != "src/new/file.go" {
		t.Errorf("newPath = %q, want %q", newPath, "src/new/file.go")
	}
}

func TestSplitRenamePath_NoRename(t *testing.T) {
	oldPath, newPath := splitRenamePath("src/file.go")
	if oldPath != "src/file.go" {
		t.Errorf("oldPath = %q, want %q", oldPath, "src/file.go")
	}
	if newPath != "src/file.go" {
		t.Errorf("newPath = %q, want %q", newPath, "src/file.go")
	}
}

func TestSumStats(t *testing.T) {
	files := []FileDiff{
		{Additions: 10, Deletions: 5},
		{Additions: 3, Deletions: 1},
		{Additions: 0, Deletions: 0, Binary: true},
	}

	add, del := sumStats(files)
	if add != 13 {
		t.Errorf("totalAdd = %d, want 13", add)
	}
	if del != 6 {
		t.Errorf("totalDel = %d, want 6", del)
	}
}

func TestSumStats_Empty(t *testing.T) {
	add, del := sumStats(nil)
	if add != 0 || del != 0 {
		t.Errorf("sumStats(nil) = (%d, %d), want (0, 0)", add, del)
	}
}

func TestFilterGoFiles(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1,2 @@
 # Title
+Some text
diff --git a/utils.go b/utils.go
--- a/utils.go
+++ b/utils.go
@@ -1 +1,2 @@
 package main
+func helper() {}`

	result := filterGoFiles(diff)

	if !containsStr(result, "main.go") {
		t.Error("filterGoFiles should include main.go")
	}
	if !containsStr(result, "utils.go") {
		t.Error("filterGoFiles should include utils.go")
	}
	if containsStr(result, "README.md") {
		t.Error("filterGoFiles should NOT include README.md")
	}
}

func TestFilterGoFiles_Empty(t *testing.T) {
	result := filterGoFiles("")
	if result != "" {
		t.Errorf("filterGoFiles(\"\") = %q, want empty string", result)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
