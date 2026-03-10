package brief

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SeanoChang/cubit/internal/claude"
)

const memoryPassPrompt = `You just completed a work session. Here is:

1. Your current brief.md:
%s

2. Tail of the session output:
%s

3. Recent log entries:
%s

Rewrite brief.md to reflect the current state.
Rules:
- Keep it under 30k tokens
- Preserve key decisions (don't drop locked items)
- Update "Current state" to reflect where things actually are
- Move completed items from "Open threads" to "Recent work"
- Add new open threads discovered this session
- Drop stale information that's no longer relevant
- Output ONLY the new brief.md content, nothing else.`

// RunMemoryPass rewrites memory/brief.md via a cheap LLM call.
// It assembles a rewrite prompt from the old brief, session output, and log,
// sends it to claude, and overwrites brief.md with the response.
func RunMemoryPass(ctx context.Context, agentDir, rawOutput string, opts claude.RunnerOpts) error {
	prompt := buildMemoryPrompt(agentDir, rawOutput)

	result, err := claude.Prompt(ctx, prompt, opts)
	if err != nil {
		return fmt.Errorf("memory pass: %w", err)
	}

	briefPath := filepath.Join(agentDir, "memory", "brief.md")
	if err := os.WriteFile(briefPath, []byte(strings.TrimSpace(result)+"\n"), 0o644); err != nil {
		return fmt.Errorf("memory pass: write brief.md: %w", err)
	}

	return nil
}

// buildMemoryPrompt assembles the rewrite prompt from agent files and session output.
func buildMemoryPrompt(agentDir, rawOutput string) string {
	oldBrief := readFile(filepath.Join(agentDir, "memory", "brief.md"))
	truncatedOutput := tail(rawOutput, 200)
	logContent := tail(readFile(filepath.Join(agentDir, "memory", "log.md")), 50)

	return fmt.Sprintf(memoryPassPrompt, oldBrief, truncatedOutput, logContent)
}

const refreshPrompt = `You are refreshing your working memory from scratch.
Below are your recent session journals and log entries.
Write a new brief.md that captures the current state of your work.

1. Recent session journals:
%s

2. Recent log entries:
%s

Write brief.md from scratch based on these sources.
Rules:
- Keep it under 30k tokens
- Include sections: Current state, Key decisions, Open threads, Recent work
- Synthesize across journals — don't just concatenate
- Capture what matters NOW, drop stale context
- Output ONLY the new brief.md content, nothing else.`

// RunRefresh rebuilds memory/brief.md from scratch using recent session
// journals and log entries. Unlike RunMemoryPass, it does not carry over
// the old brief — it reads raw sources and synthesizes a fresh summary.
func RunRefresh(ctx context.Context, agentDir string, opts claude.RunnerOpts, numJournals int) error {
	prompt := buildRefreshPrompt(agentDir, numJournals)

	result, err := claude.Prompt(ctx, prompt, opts)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}

	briefPath := filepath.Join(agentDir, "memory", "brief.md")
	if err := os.WriteFile(briefPath, []byte(strings.TrimSpace(result)+"\n"), 0o644); err != nil {
		return fmt.Errorf("refresh: write brief.md: %w", err)
	}

	return nil
}

// buildRefreshPrompt assembles the fresh-start prompt from recent journals and log.
func buildRefreshPrompt(agentDir string, numJournals int) string {
	journals := recentJournals(agentDir, numJournals)
	logContent := tail(readFile(filepath.Join(agentDir, "memory", "log.md")), 50)

	return fmt.Sprintf(refreshPrompt, journals, logContent)
}

// recentJournals reads the last n journal files from memory/sessions/*.md,
// sorted by filename, and joins them with a separator.
func recentJournals(agentDir string, n int) string {
	pattern := filepath.Join(agentDir, "memory", "sessions", "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	sort.Strings(matches)

	// Take only the last n.
	if len(matches) > n {
		matches = matches[len(matches)-n:]
	}

	var parts []string
	for _, path := range matches {
		content := readFile(path)
		if content != "" {
			parts = append(parts, content)
		}
	}

	return strings.Join(parts, "\n\n---\n\n")
}

const consolidationPrompt = `You are consolidating observations from parallel workers into the agent's working memory.

1. Current brief.md:
%s

2. Worker observations:
%s

Rewrite brief.md to incorporate the key learnings from all workers.
Rules:
- Keep it under 30k tokens
- Merge observations — don't just append them
- Preserve existing key decisions and open threads
- Add new findings, patterns, and decisions from the workers
- Drop stale information superseded by worker results
- Output ONLY the new brief.md content, nothing else.`

// RunConsolidation reads all scratch/*-observations.md files and consolidates
// them into memory/brief.md via a single LLM call. Called once after drain
// completes all tasks. No-op if no observation files exist.
func RunConsolidation(ctx context.Context, agentDir string, opts claude.RunnerOpts) error {
	pattern := filepath.Join(agentDir, "scratch", "*-observations.md")
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) == 0 {
		return nil
	}

	sort.Strings(files)

	var observations []string
	for _, f := range files {
		content := readFile(f)
		if content != "" {
			observations = append(observations, content)
		}
	}
	if len(observations) == 0 {
		return nil
	}

	currentBrief := readFile(filepath.Join(agentDir, "memory", "brief.md"))
	prompt := fmt.Sprintf(consolidationPrompt, currentBrief, strings.Join(observations, "\n\n---\n\n"))

	result, err := claude.Prompt(ctx, prompt, opts)
	if err != nil {
		return fmt.Errorf("consolidation: %w", err)
	}

	briefPath := filepath.Join(agentDir, "memory", "brief.md")
	if err := os.WriteFile(briefPath, []byte(strings.TrimSpace(result)+"\n"), 0o644); err != nil {
		return fmt.Errorf("consolidation: write brief.md: %w", err)
	}

	return nil
}
