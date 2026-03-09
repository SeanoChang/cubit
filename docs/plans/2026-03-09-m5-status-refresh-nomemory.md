# M5: Status, Refresh, --no-memory — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ship `cubit status`, `cubit refresh`, and `--no-memory` flag to complete M5.

**Architecture:** Three independent features. `status` reads existing state (queue, brief, config) and prints a summary. `refresh` adds a new `RunRefresh()` function in `internal/brief/memory.go` that rewrites `brief.md` from scratch using recent journals + log (no prior brief carried over). `--no-memory` is a flag on `prompt` and `run` that skips `RunMemoryPass()`.

**Tech Stack:** Go 1.26, Cobra, existing `internal/brief/` and `internal/queue/` packages.

---

## Task 1: `cubit status` command

**Files:**
- Create: `cmd/status.go`
- Modify: `cmd/root.go:36-80` (register command)
- Test: manual (`go build && ./cubit status`)

**Step 1: Create `cmd/status.go`**

```go
package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show agent state, queue depth, and brief size",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Agent:   %s\n", cfg.Agent)

		// Active task
		task, err := q.Active()
		if err != nil {
			return err
		}
		if task != nil {
			fmt.Printf("Active:  %03d — %s\n", task.ID, task.Title)
		} else {
			fmt.Println("Active:  (none)")
		}

		// Queue depth
		tasks, err := q.List()
		if err != nil {
			return err
		}
		fmt.Printf("Queue:   %d pending\n", len(tasks))

		// Brief token size
		sections := brief.Sections(cfg.AgentDir())
		total := 0
		for _, s := range sections {
			total += brief.EstimateTokens(s.Content)
		}

		// Find brief.md section specifically
		briefTokens := "(none)"
		for _, s := range sections {
			if s.Label == "Brief" && s.Content != "" {
				briefTokens = brief.FormatTokens(s.Content)
				break
			}
		}
		fmt.Printf("Brief:   %s\n", briefTokens)

		// Total injection size
		fmt.Printf("Inject:  ~%d tokens total\n", total)

		// Scratch files
		planContent := readFileOpt(filepath.Join(cfg.AgentDir(), "scratch", "plan.md"))
		notesContent := readFileOpt(filepath.Join(cfg.AgentDir(), "scratch", "notes.md"))
		var scratch []string
		if planContent != "" {
			scratch = append(scratch, "plan.md")
		}
		if notesContent != "" {
			scratch = append(scratch, "notes.md")
		}
		if len(scratch) > 0 {
			fmt.Printf("Scratch: %s\n", strings.Join(scratch, ", "))
		}

		return nil
	},
}

func readFileOpt(path string) string {
	// Reuse the same pattern as brief.readFile but in cmd package.
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
```

Note: `readFileOpt` needs `"os"` import. The full import list is: `fmt`, `os`, `path/filepath`, `strings`, `brief`, `cobra`.

**Step 2: Register in `cmd/root.go`**

Add after the `cubit run` block (line 79):

```go
	// cubit status
	rootCmd.AddCommand(statusCmd)
```

**Step 3: Build and test**

```bash
go build -o cubit . && ./cubit status
```

Expected output (with noah agent initialized):
```
Agent:   noah
Active:  (none)
Queue:   0 pending
Brief:   ~NNN tokens
Inject:  ~NNN tokens total
```

**Step 4: Commit**

```bash
git add cmd/status.go cmd/root.go
git commit -m "feat(m5): add cubit status command"
```

---

## Task 2: `RunRefresh()` in `internal/brief/memory.go`

**Files:**
- Modify: `internal/brief/memory.go` (add `RunRefresh`, `buildRefreshPrompt`, `recentJournals`)
- Create: `internal/brief/refresh_test.go`

**Step 1: Write the failing tests in `internal/brief/refresh_test.go`**

```go
package brief

import (
	"strings"
	"testing"
)

func TestBuildRefreshPrompt(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, dir, "memory/sessions/2026-03-08T10:00.md", "## What happened\nFixed auth bug.")
	writeTestFile(t, dir, "memory/sessions/2026-03-09T14:00.md", "## What happened\nShipped queue.")
	writeTestFile(t, dir, "memory/log.md", "2026-03-08 fixed login\n2026-03-09 shipped queue")

	prompt := buildRefreshPrompt(dir)

	// Should contain journal content.
	if !strings.Contains(prompt, "Fixed auth bug.") {
		t.Error("prompt missing journal content")
	}
	if !strings.Contains(prompt, "Shipped queue.") {
		t.Error("prompt missing second journal content")
	}

	// Should contain log content.
	if !strings.Contains(prompt, "shipped queue") {
		t.Error("prompt missing log content")
	}

	// Should have refresh-specific instructions (not the post-session prompt).
	if !strings.Contains(prompt, "from scratch") {
		t.Error("prompt missing 'from scratch' instruction")
	}

	// Should NOT contain old brief (fresh start).
	if strings.Contains(prompt, "Your current brief.md") {
		t.Error("refresh prompt should not reference old brief")
	}
}

func TestBuildRefreshPromptNoJournals(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "memory/log.md", "some log entry")

	prompt := buildRefreshPrompt(dir)

	if !strings.Contains(prompt, "some log entry") {
		t.Error("prompt missing log content when no journals")
	}

	// Should still have instructions.
	if !strings.Contains(prompt, "from scratch") {
		t.Error("prompt missing instructions when no journals")
	}
}

func TestRecentJournals(t *testing.T) {
	dir := t.TempDir()

	// Create 7 journals — should only get last 5.
	for i := 1; i <= 7; i++ {
		name := "memory/sessions/2026-03-0" + string(rune('0'+i)) + "T10:00.md"
		writeTestFile(t, dir, name, "journal "+string(rune('0'+i)))
	}

	result := recentJournals(dir, 5)

	// Should contain journals 3-7 (last 5).
	if strings.Contains(result, "journal 1") || strings.Contains(result, "journal 2") {
		t.Error("recentJournals included too-old journals")
	}
	if !strings.Contains(result, "journal 7") {
		t.Error("recentJournals missing most recent journal")
	}
}

func TestRecentJournalsEmpty(t *testing.T) {
	dir := t.TempDir()

	result := recentJournals(dir, 5)
	if result != "" {
		t.Errorf("recentJournals on empty dir = %q, want empty", result)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/brief/ -run TestBuildRefreshPrompt -v
```

Expected: FAIL — `buildRefreshPrompt` undefined.

**Step 3: Implement in `internal/brief/memory.go`**

Add the refresh prompt constant and three functions:

```go
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

// RunRefresh rewrites brief.md from scratch using recent journals and log.
// Unlike RunMemoryPass, it does not carry over the old brief.
func RunRefresh(agentDir, model string) error {
	prompt := buildRefreshPrompt(agentDir)

	result, err := claude.Prompt(prompt, model)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}

	briefPath := filepath.Join(agentDir, "memory", "brief.md")
	if err := os.WriteFile(briefPath, []byte(strings.TrimSpace(result)+"\n"), 0o644); err != nil {
		return fmt.Errorf("refresh: write brief.md: %w", err)
	}

	return nil
}

// buildRefreshPrompt assembles a fresh-start prompt from journals and log.
func buildRefreshPrompt(agentDir string) string {
	journals := recentJournals(agentDir, 5)
	logContent := tail(readFile(filepath.Join(agentDir, "memory", "log.md")), 50)
	return fmt.Sprintf(refreshPrompt, journals, logContent)
}

// recentJournals reads the last n session journal files (sorted by name).
func recentJournals(agentDir string, n int) string {
	pattern := filepath.Join(agentDir, "memory", "sessions", "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	sort.Strings(matches)
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
```

Note: `memory.go` needs `"sort"` added to imports.

**Step 4: Run tests to verify they pass**

```bash
cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/brief/ -v
```

Expected: all tests PASS (existing + new).

**Step 5: Commit**

```bash
git add internal/brief/memory.go internal/brief/refresh_test.go
git commit -m "feat(m5): add RunRefresh for fresh brief.md rewrite from journals"
```

---

## Task 3: `cubit refresh` command

**Files:**
- Create: `cmd/refresh.go`
- Modify: `cmd/root.go` (register command)

**Step 1: Create `cmd/refresh.go`**

```go
package cmd

import (
	"fmt"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Rewrite brief.md from scratch using recent journals and log",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Refreshing brief.md from journals + log...")

		if err := brief.RunRefresh(cfg.AgentDir(), cfg.Claude.MemoryModel); err != nil {
			return err
		}

		fmt.Println("Done. brief.md rewritten.")
		return nil
	},
}
```

**Step 2: Register in `cmd/root.go`**

Add after the `cubit status` registration:

```go
	// cubit refresh
	rootCmd.AddCommand(refreshCmd)
```

**Step 3: Build and verify**

```bash
go build -o cubit . && ./cubit refresh
```

Expected: calls LLM, rewrites `~/.ark/cubit/noah/memory/brief.md`.

**Step 4: Commit**

```bash
git add cmd/refresh.go cmd/root.go
git commit -m "feat(m5): add cubit refresh command"
```

---

## Task 4: `--no-memory` flag on `prompt` and `run`

**Files:**
- Modify: `cmd/root.go` (register flag)
- Modify: `cmd/prompt.go` (check flag)
- Modify: `cmd/run.go` (check flag)

**Step 1: Register flag in `cmd/root.go`**

Add to the `cubit prompt` section (after line 71):

```go
	// cubit prompt "message" [--no-memory]
	promptCmd.Flags().Bool("no-memory", false, "Skip the post-session memory pass")
	rootCmd.AddCommand(promptCmd)
```

Add to the `cubit run` section (after line 78):

```go
	runCmd.Flags().Bool("no-memory", false, "Skip the post-session memory pass")
```

**Step 2: Guard memory pass in `cmd/prompt.go`**

Replace the memory pass block (lines 26-28):

```go
		noMemory, _ := cmd.Flags().GetBool("no-memory")
		if !noMemory {
			if err := brief.RunMemoryPass(cfg.AgentDir(), result, cfg.Claude.MemoryModel); err != nil {
				fmt.Printf("warning: memory pass failed: %v\n", err)
			}
		}
```

**Step 3: Guard memory pass in `cmd/run.go`**

Read `no-memory` flag at the top of RunE (alongside `once` and `cooldown`):

```go
		noMemory, _ := cmd.Flags().GetBool("no-memory")
```

Replace the memory pass block (lines 72-75):

```go
			if !noMemory {
				if err := brief.RunMemoryPass(cfg.AgentDir(), result, cfg.Claude.MemoryModel); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: memory pass failed: %v\n", err)
				}
			}
```

**Step 4: Build and verify**

```bash
go build -o cubit .
./cubit prompt "hello" --no-memory   # should skip memory pass
./cubit run --once --no-memory       # should skip memory pass
```

**Step 5: Commit**

```bash
git add cmd/root.go cmd/prompt.go cmd/run.go
git commit -m "feat(m5): add --no-memory flag to prompt and run"
```

---

## Task 5: Final build + verify all commands

**Step 1: Run all tests**

```bash
cd /Users/seanochang/dev/projects/agents/cubit && go test ./...
```

Expected: all PASS.

**Step 2: Build and smoke test**

```bash
go build -o cubit .
./cubit status
./cubit --help          # verify status and refresh show up
./cubit prompt --help   # verify --no-memory shows up
./cubit run --help      # verify --no-memory shows up
```

**Step 3: Final commit (if any fixups needed)**
