package brief

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	// 10 words * 1.3 = 13
	text := "one two three four five six seven eight nine ten"
	got := EstimateTokens(text)
	if got != 13 {
		t.Errorf("EstimateTokens(10 words) = %d, want 13", got)
	}
}

func TestEstimateTokensEmpty(t *testing.T) {
	got := EstimateTokens("")
	if got != 0 {
		t.Errorf("EstimateTokens(\"\") = %d, want 0", got)
	}
}

func TestBuild(t *testing.T) {
	dir := t.TempDir()

	// Create all expected files.
	writeTestFile(t, dir, "identity/FLUCTLIGHT.md", "You are Noah.")
	writeTestFile(t, dir, "USER.md", "Sean is a developer.")
	writeTestFile(t, dir, "GOALS.md", "Ship the MVP.")
	writeTestFile(t, dir, "memory/brief.md", "Last session we fixed auth.")
	writeTestFile(t, dir, "queue/.doing", "Implement the queue drain.")
	writeTestFile(t, dir, "scratch/plan.md", "Step 1: read code.")

	result := Build(dir)

	// Every section should be present.
	for _, want := range []string{
		"You are Noah.",
		"Sean is a developer.",
		"Ship the MVP.",
		"Last session we fixed auth.",
		"## Active Task\nImplement the queue drain.",
		"## Current Plan\nStep 1: read code.",
	} {
		if !strings.Contains(result, want) {
			t.Errorf("Build() missing %q", want)
		}
	}

	// Sections joined by separator.
	if strings.Count(result, "\n\n---\n\n") != 5 {
		t.Errorf("expected 5 separators, got %d", strings.Count(result, "\n\n---\n\n"))
	}
}

func TestBuildSkipsMissingFiles(t *testing.T) {
	dir := t.TempDir()

	// Only FLUCTLIGHT exists.
	writeTestFile(t, dir, "identity/FLUCTLIGHT.md", "You are Noah.")

	result := Build(dir)

	if !strings.Contains(result, "You are Noah.") {
		t.Error("Build() missing FLUCTLIGHT content")
	}

	// No separators since there's only one section.
	if strings.Contains(result, "---") {
		t.Error("Build() should have no separators with a single file")
	}

	// No phantom sections from missing files.
	if strings.Contains(result, "Active Task") {
		t.Error("Build() should not include Active Task when .doing is missing")
	}
	if strings.Contains(result, "Current Plan") {
		t.Error("Build() should not include Current Plan when plan.md is missing")
	}
}

func TestBuildWithActiveTask(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, dir, "queue/.doing", "Fix the login bug.")

	result := Build(dir)

	want := "## Active Task\nFix the login bug."
	if !strings.Contains(result, want) {
		t.Errorf("Build() = %q, want it to contain %q", result, want)
	}
}

func TestSections(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, dir, "identity/FLUCTLIGHT.md", "identity content")
	writeTestFile(t, dir, "GOALS.md", "goals content")

	sections := Sections(dir)

	if len(sections) != 6 {
		t.Fatalf("Sections() returned %d sections, want 6", len(sections))
	}

	// FLUCTLIGHT should have content.
	if sections[0].Label != "FLUCTLIGHT" || sections[0].Content != "identity content" {
		t.Errorf("section[0] = %+v, want FLUCTLIGHT with content", sections[0])
	}

	// USER should be empty (missing file).
	if sections[1].Label != "USER" || sections[1].Content != "" {
		t.Errorf("section[1] = %+v, want empty USER", sections[1])
	}

	// GOALS should have content.
	if sections[2].Label != "GOALS" || sections[2].Content != "goals content" {
		t.Errorf("section[2] = %+v, want GOALS with content", sections[2])
	}
}

func TestFormatTokens(t *testing.T) {
	if got := FormatTokens(""); got != "(none)" {
		t.Errorf("FormatTokens(\"\") = %q, want \"(none)\"", got)
	}

	got := FormatTokens("one two three four five six seven eight nine ten")
	if !strings.HasPrefix(got, "~") || !strings.HasSuffix(got, "tokens") {
		t.Errorf("FormatTokens(10 words) = %q, want ~N tokens format", got)
	}
}

func TestTail(t *testing.T) {
	text := "line1\nline2\nline3\nline4\nline5"

	got := tail(text, 3)
	want := "line3\nline4\nline5"
	if got != want {
		t.Errorf("tail(5 lines, 3) = %q, want %q", got, want)
	}

	// Fewer lines than n returns everything.
	got = tail(text, 10)
	if got != text {
		t.Errorf("tail(5 lines, 10) = %q, want original text", got)
	}
}

// writeTestFile creates a file under dir with the given relative path and content.
func writeTestFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
