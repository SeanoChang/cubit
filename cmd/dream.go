package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const dreamPrompt = `You are performing memory consolidation ("dreaming") for an agent workspace.

## Your Task

Reorganize this agent's memory from a monolithic MEMORY.md into a clean index + topic files in memory/.

## Four Phases

### Phase 1 — Orient
Read the current memory state below. Understand what exists, what's stale, what's important.

### Phase 2 — Gather Signal
Identify high-value content: architecture docs, active projects, operational knowledge, user preferences, decision rationale. Also identify stale content: completed projects, outdated market data, resolved issues, session-specific debugging notes.

### Phase 3 — Consolidate
- Merge overlapping entries (if the same topic appears in multiple places, unify it)
- Resolve contradictions (keep the newer information)
- Convert relative dates ("yesterday", "last week") to absolute dates
- Prune completed/stale items — don't keep them "just in case"
- Separate evergreen knowledge from volatile state

### Phase 4 — Write Output
Create topic files and a clean index. Output your result as a structured document with clear file markers.

## Output Format

Output the consolidated memory as a series of files, each marked with a FILE: header:

FILE: MEMORY.md
<index content — pointers to topic files, max ~100 lines>

FILE: memory/<topic>.md
<topic content>

FILE: memory/<another-topic>.md
<topic content>

...repeat for each topic file...

## Rules

- MEMORY.md is the INDEX — one-line pointers to topic files, not content
- Each index entry: "- [Title](memory/file.md) — brief description"
- Topic file names should be kebab-case and descriptive (e.g., architecture.md, market-state.md, decisions.md)
- Keep total topic files reasonable (3-10 files, not 30)
- Preserve ALL important information — consolidation means reorganizing, not deleting knowledge
- If content is truly stale (completed project, resolved bug), summarize it in 1-2 lines rather than keeping the full block
- User preferences, communication style, and operational notes are HIGH value — always keep these
- Decision rationale ("chose X because Y") is HIGH value — always keep
- Do NOT invent information. Only reorganize what's provided.
- Output ONLY the file contents in the format above. No preamble, no explanation.

## Current Memory State

### MEMORY.md
%s
### Topic Files in memory/
%s
### log.md (recent entries)
%s`

var dreamCmd = &cobra.Command{
	Use:   "dream",
	Short: "Consolidate agent memory — reorganize MEMORY.md into index + topic files",
	Long: `Runs LLM-powered memory consolidation for an agent workspace.

Reads MEMORY.md and memory/*.md, invokes Claude to reorganize into
a clean index (MEMORY.md) + topic files (memory/*.md).

The original MEMORY.md is archived to memory/archive/<timestamp>.md.

Examples:
  cubit dream                    # consolidate default agent's memory
  cubit dream --dry-run          # show what would change without applying
  cubit dream --include-log      # include log.md for temporal context
  cubit dream --agent neo        # consolidate a specific agent`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !agentExplicit {
			return fmt.Errorf("agent not specified — use --agent <name> or run from inside an agent directory")
		}

		agentDir := cfg.AgentDir()
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		includeLog, _ := cmd.Flags().GetBool("include-log")

		// Check claude CLI is available
		if _, err := exec.LookPath("claude"); err != nil {
			return fmt.Errorf("claude CLI not found — required for memory consolidation")
		}

		// Read MEMORY.md
		memoryPath := filepath.Join(agentDir, "MEMORY.md")
		memoryData, err := os.ReadFile(memoryPath)
		if err != nil {
			return fmt.Errorf("reading MEMORY.md: %w", err)
		}
		memoryContent := string(memoryData)

		lines := len(strings.Split(strings.TrimRight(memoryContent, "\n"), "\n"))
		fmt.Printf("Reading MEMORY.md (%d lines)\n", lines)

		// Read existing topic files in memory/
		memDir := filepath.Join(agentDir, "memory")
		topicContent := readTopicFiles(memDir)

		// Optionally read log.md
		logContent := "(not included — use --include-log to add)"
		if includeLog {
			logPath := filepath.Join(agentDir, "log.md")
			if data, err := os.ReadFile(logPath); err == nil {
				// Take last 50 lines for context
				logLines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
				if len(logLines) > 50 {
					logLines = logLines[len(logLines)-50:]
				}
				logContent = strings.Join(logLines, "\n")
				fmt.Printf("Reading log.md (last %d lines)\n", len(logLines))
			}
		}

		// Build prompt
		prompt := fmt.Sprintf(dreamPrompt, memoryContent, topicContent, logContent)

		fmt.Println("Consolidating memory with Claude...")

		// Invoke claude
		var stderr strings.Builder
		claude := exec.Command("claude", "-p", "--output-format", "text", prompt)
		claude.Dir = agentDir
		claude.Stderr = &stderr
		output, err := claude.Output()
		if err != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail != "" {
				return fmt.Errorf("claude consolidation failed: %w\n%s", err, detail)
			}
			return fmt.Errorf("claude consolidation failed: %w", err)
		}

		result := string(output)

		// Parse output into files
		files := parseDreamOutput(result)
		if len(files) == 0 {
			preview := result
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			return fmt.Errorf("consolidation produced no parseable files (expected FILE: markers)\nClaude output preview:\n%s", preview)
		}

		// Check we got a MEMORY.md in the output
		hasIndex := false
		for _, f := range files {
			if f.path == "MEMORY.md" {
				hasIndex = true
				break
			}
		}
		if !hasIndex {
			return fmt.Errorf("consolidation output missing MEMORY.md index")
		}

		if dryRun {
			fmt.Println("\n--- DRY RUN — no changes applied ---")
			for _, f := range files {
				lines := len(strings.Split(strings.TrimRight(f.content, "\n"), "\n"))
				fmt.Printf("  write %-35s %4d lines\n", f.path, lines)
			}
			// Show orphans that would be removed
			outputPaths := make(map[string]bool)
			for _, f := range files {
				outputPaths[f.path] = true
			}
			filepath.WalkDir(memDir, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil || d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(memDir, path)
				if strings.HasPrefix(rel, "archive/") || strings.HasPrefix(rel, "archive\\") {
					return nil
				}
				memPath := filepath.Join("memory", rel)
				if !outputPaths[memPath] {
					fmt.Printf("  remove orphan: memory/%s\n", rel)
				}
				return nil
			})
			fmt.Printf("\n  %d files would be written\n", len(files))
			return nil
		}

		// Archive original MEMORY.md
		archiveDir := filepath.Join(memDir, "archive")
		if err := os.MkdirAll(archiveDir, 0o755); err != nil {
			return fmt.Errorf("creating archive dir: %w", err)
		}
		timestamp := time.Now().Format("2006-01-02T150405")
		archivePath := filepath.Join(archiveDir, timestamp+".md")
		if err := os.WriteFile(archivePath, memoryData, 0o644); err != nil {
			return fmt.Errorf("archiving MEMORY.md: %w", err)
		}
		fmt.Printf("Archived original → memory/archive/%s.md\n", timestamp)

		// Ensure memory/ dir exists
		if err := os.MkdirAll(memDir, 0o755); err != nil {
			return fmt.Errorf("creating memory dir: %w", err)
		}

		// Write consolidated files
		for _, f := range files {
			targetPath := filepath.Join(agentDir, f.path)

			// Ensure parent dir exists for nested paths
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("creating directory for %s: %w", f.path, err)
			}

			if err := os.WriteFile(targetPath, []byte(f.content), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", f.path, err)
			}

			lines := len(strings.Split(strings.TrimRight(f.content, "\n"), "\n"))
			fmt.Printf("  wrote %-35s %4d lines\n", f.path, lines)
		}

		// Clean up orphaned topic files not in the output set
		outputPaths := make(map[string]bool)
		for _, f := range files {
			outputPaths[f.path] = true
		}
		var removed int
		filepath.WalkDir(memDir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(memDir, path)
			if strings.HasPrefix(rel, "archive/") || strings.HasPrefix(rel, "archive\\") {
				return nil
			}
			memPath := filepath.Join("memory", rel)
			if !outputPaths[memPath] {
				os.Remove(path)
				fmt.Printf("  removed orphan: memory/%s\n", rel)
				removed++
			}
			return nil
		})

		fmt.Printf("\nDream complete. %d files written", len(files))
		if removed > 0 {
			fmt.Printf(", %d orphans removed", removed)
		}
		fmt.Println(".")
		return nil
	},
}

type dreamFile struct {
	path    string
	content string
}

// parseDreamOutput extracts FILE: markers from Claude's output.
func parseDreamOutput(output string) []dreamFile {
	var files []dreamFile
	lines := strings.Split(output, "\n")

	var currentPath string
	var currentContent strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "FILE: ") {
			// Save previous file if any
			if currentPath != "" {
				files = append(files, dreamFile{
					path:    currentPath,
					content: strings.TrimSpace(currentContent.String()) + "\n",
				})
			}
			currentPath = strings.TrimPrefix(trimmed, "FILE: ")
			currentContent.Reset()
			continue
		}

		if currentPath != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// Save last file
	if currentPath != "" {
		files = append(files, dreamFile{
			path:    currentPath,
			content: strings.TrimSpace(currentContent.String()) + "\n",
		})
	}

	// Security: validate all paths are within expected locations
	var safe []dreamFile
	for _, f := range files {
		clean := filepath.Clean(f.path)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue // skip paths that escape the agent dir
		}
		// Only allow MEMORY.md and memory/* paths
		if clean == "MEMORY.md" || strings.HasPrefix(clean, "memory/") {
			// Don't allow writing into memory/archive/
			if strings.HasPrefix(clean, "memory/archive/") {
				continue
			}
			f.path = clean
			safe = append(safe, f)
		}
	}

	return safe
}

// readTopicFiles reads all files in the memory/ directory (excluding archive/).
func readTopicFiles(memDir string) string {
	if _, err := os.Stat(memDir); os.IsNotExist(err) {
		return "(no memory/ directory)"
	}

	var content strings.Builder
	empty := true

	filepath.WalkDir(memDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(memDir, path)
		// Skip archive files
		if strings.HasPrefix(rel, "archive/") || strings.HasPrefix(rel, "archive\\") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content.WriteString(fmt.Sprintf("\n#### memory/%s\n%s\n", rel, string(data)))
		empty = false
		return nil
	})

	if empty {
		return "(no topic files yet)"
	}
	return content.String()
}

func init() {
	dreamCmd.Flags().Bool("dry-run", false, "Show what would change without applying")
	dreamCmd.Flags().Bool("include-log", false, "Include recent log.md entries for temporal context")
}
