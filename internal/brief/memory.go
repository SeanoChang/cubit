package brief

import (
	"fmt"
	"os"
	"path/filepath"
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
func RunMemoryPass(agentDir, rawOutput, model string) error {
	prompt := buildMemoryPrompt(agentDir, rawOutput)

	result, err := claude.Prompt(prompt, model)
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
