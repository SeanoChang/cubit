package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestLineCount(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"one line", 1},
		{"line1\nline2\nline3", 3},
		{"line1\nline2\n", 2},
		{"", 1},
	}
	for _, tt := range tests {
		got := lineCount(tt.input)
		if got != tt.want {
			t.Errorf("lineCount(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestListTopicFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create topic files
	os.WriteFile(filepath.Join(tmp, "architecture.md"), []byte("# Arch"), 0o644)
	os.WriteFile(filepath.Join(tmp, "decisions.md"), []byte("# Decisions"), 0o644)

	// Create archive/ — should be excluded
	archiveDir := filepath.Join(tmp, "archive")
	os.MkdirAll(archiveDir, 0o755)
	os.WriteFile(filepath.Join(archiveDir, "2026-01-01.md"), []byte("old"), 0o644)

	got := listTopicFiles(tmp)
	sort.Strings(got)

	want := []string{
		filepath.Join(tmp, "architecture.md"),
		filepath.Join(tmp, "decisions.md"),
	}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("got %d files, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("file[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestListTopicFilesEmpty(t *testing.T) {
	tmp := t.TempDir()
	got := listTopicFiles(tmp)
	if len(got) != 0 {
		t.Errorf("expected 0 files, got %d", len(got))
	}
}
