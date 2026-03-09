package brief

import (
	"strings"
	"testing"
)

func TestBuildMemoryPrompt(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, dir, "memory/brief.md", "# Brief\nWe shipped auth last session.")
	writeTestFile(t, dir, "memory/log.md", "2026-03-07 fixed login\n2026-03-08 added queue")

	prompt := buildMemoryPrompt(dir, "agent output here")

	// Should contain old brief content.
	if !strings.Contains(prompt, "We shipped auth last session.") {
		t.Error("prompt missing old brief content")
	}

	// Should contain raw output.
	if !strings.Contains(prompt, "agent output here") {
		t.Error("prompt missing raw output")
	}

	// Should contain log content.
	if !strings.Contains(prompt, "added queue") {
		t.Error("prompt missing log content")
	}

	// Should contain the rewrite instructions.
	if !strings.Contains(prompt, "Rewrite brief.md") {
		t.Error("prompt missing rewrite instructions")
	}
}

func TestBuildMemoryPromptMissingFiles(t *testing.T) {
	dir := t.TempDir()

	// No brief.md or log.md exist — should not panic.
	prompt := buildMemoryPrompt(dir, "some output")

	if !strings.Contains(prompt, "some output") {
		t.Error("prompt missing raw output when files are absent")
	}

	// Should still have the rewrite instructions.
	if !strings.Contains(prompt, "Rewrite brief.md") {
		t.Error("prompt missing rewrite instructions when files are absent")
	}
}

func TestBuildMemoryPromptTruncatesOutput(t *testing.T) {
	// Generate 250 lines of output.
	var lines []string
	for i := 1; i <= 250; i++ {
		lines = append(lines, "line content")
	}
	longOutput := strings.Join(lines, "\n")

	dir := t.TempDir()
	writeTestFile(t, dir, "memory/brief.md", "old brief")
	writeTestFile(t, dir, "memory/log.md", "log entry")

	prompt := buildMemoryPrompt(dir, longOutput)

	// The raw output section should contain only the last 200 lines,
	// not all 250. Count occurrences of "line content" in the output section.
	outputCount := strings.Count(prompt, "line content")
	if outputCount != 200 {
		t.Errorf("expected 200 lines of output in prompt, got %d", outputCount)
	}
}

func TestBuildMemoryPromptTruncatesLog(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "memory/brief.md", "brief")

	// Generate 80 lines of log.
	var lines []string
	for i := 1; i <= 80; i++ {
		lines = append(lines, "log line")
	}
	writeTestFile(t, dir, "memory/log.md", strings.Join(lines, "\n"))

	prompt := buildMemoryPrompt(dir, "output")

	// Should only have last 50 lines of log.
	logCount := strings.Count(prompt, "log line")
	if logCount != 50 {
		t.Errorf("expected 50 log lines in prompt, got %d", logCount)
	}
}
