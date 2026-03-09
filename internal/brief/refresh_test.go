package brief

import (
	"strings"
	"testing"
)

func TestBuildRefreshPrompt(t *testing.T) {
	dir := t.TempDir()

	// Create journal files.
	writeTestFile(t, dir, "memory/sessions/2026-03-08-abc.md", "Session 1: Fixed auth bug.")
	writeTestFile(t, dir, "memory/sessions/2026-03-09-def.md", "Session 2: Added queue drain.")

	// Create log.
	writeTestFile(t, dir, "memory/log.md", "2026-03-08 fixed auth\n2026-03-09 added queue")

	prompt := buildRefreshPrompt(dir, 5)

	// Should contain journal content.
	if !strings.Contains(prompt, "Session 1: Fixed auth bug.") {
		t.Error("buildRefreshPrompt() missing journal 1 content")
	}
	if !strings.Contains(prompt, "Session 2: Added queue drain.") {
		t.Error("buildRefreshPrompt() missing journal 2 content")
	}

	// Should contain log content.
	if !strings.Contains(prompt, "2026-03-08 fixed auth") {
		t.Error("buildRefreshPrompt() missing log content")
	}

	// Should contain "from scratch" instruction.
	if !strings.Contains(prompt, "from scratch") {
		t.Error("buildRefreshPrompt() missing 'from scratch' instruction")
	}

	// Should NOT contain old brief reference (this is a fresh start).
	if strings.Contains(prompt, "Your current brief.md") {
		t.Error("buildRefreshPrompt() should NOT reference 'Your current brief.md'")
	}
}

func TestBuildRefreshPromptNoJournals(t *testing.T) {
	dir := t.TempDir()

	// Only log, no journals.
	writeTestFile(t, dir, "memory/log.md", "2026-03-08 fixed auth")

	prompt := buildRefreshPrompt(dir, 5)

	// Should still contain log content.
	if !strings.Contains(prompt, "2026-03-08 fixed auth") {
		t.Error("buildRefreshPrompt() missing log content when no journals")
	}

	// Should contain "from scratch" instruction.
	if !strings.Contains(prompt, "from scratch") {
		t.Error("buildRefreshPrompt() missing 'from scratch' instruction")
	}
}

func TestRecentJournals(t *testing.T) {
	dir := t.TempDir()

	// Create 7 journal files — only the last 5 should be returned.
	for i, name := range []string{
		"2026-03-01-aaa.md",
		"2026-03-02-bbb.md",
		"2026-03-03-ccc.md",
		"2026-03-04-ddd.md",
		"2026-03-05-eee.md",
		"2026-03-06-fff.md",
		"2026-03-07-ggg.md",
	} {
		writeTestFile(t, dir, "memory/sessions/"+name, "Journal "+string(rune('A'+i)))
	}

	result := recentJournals(dir, 5)

	// Should contain last 5 journals (C through G).
	for _, want := range []string{"Journal C", "Journal D", "Journal E", "Journal F", "Journal G"} {
		if !strings.Contains(result, want) {
			t.Errorf("recentJournals() missing %q", want)
		}
	}

	// Should NOT contain first 2 journals.
	for _, notWant := range []string{"Journal A", "Journal B"} {
		if strings.Contains(result, notWant) {
			t.Errorf("recentJournals() should not contain %q", notWant)
		}
	}

	// Journals should be separated by ---
	if strings.Count(result, "\n\n---\n\n") != 4 {
		t.Errorf("recentJournals() expected 4 separators, got %d", strings.Count(result, "\n\n---\n\n"))
	}
}

func TestRecentJournalsEmpty(t *testing.T) {
	dir := t.TempDir()

	result := recentJournals(dir, 5)

	if result != "" {
		t.Errorf("recentJournals() on empty dir = %q, want empty string", result)
	}
}
