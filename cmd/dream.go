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

const dreamPrompt = `Consolidate this agent's MEMORY.md into memory/ topic files.

Read MEMORY.md, break it into topic files under memory/, and update MEMORY.md as an index pointing to them. Organize however you see fit. All topic files must be in memory/.

Do NOT touch memory/archive/.`

const dreamDryRunSuffix = `

DRY RUN: Do NOT create or modify any files. Just describe what you would do.`

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
  cubit dream --agent neo        # consolidate a specific agent`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !agentExplicit {
			return fmt.Errorf("agent not specified — use --agent <name> or run from inside an agent directory")
		}

		agentDir := cfg.AgentDir()
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		// Check claude CLI is available
		if _, err := exec.LookPath("claude"); err != nil {
			return fmt.Errorf("claude CLI not found — required for memory consolidation")
		}

		// Read MEMORY.md to get baseline stats
		memoryPath := filepath.Join(agentDir, "MEMORY.md")
		memoryData, err := os.ReadFile(memoryPath)
		if err != nil {
			return fmt.Errorf("reading MEMORY.md: %w", err)
		}
		origLines := lineCount(string(memoryData))
		fmt.Printf("Reading MEMORY.md (%d lines)\n", origLines)

		memDir := filepath.Join(agentDir, "memory")

		if dryRun {
			fmt.Println("Analyzing memory (dry run)...")
			var stderr strings.Builder
			claude := exec.Command("claude", "-p",
				"--output-format", "text",
				dreamPrompt+dreamDryRunSuffix)
			claude.Dir = agentDir
			claude.Stderr = &stderr
			output, err := claude.Output()
			if err != nil {
				detail := strings.TrimSpace(stderr.String())
				if detail != "" {
					return fmt.Errorf("claude dry-run failed: %w\n%s", err, detail)
				}
				return fmt.Errorf("claude dry-run failed: %w", err)
			}
			fmt.Println("\n--- DRY RUN — no changes applied ---")
			fmt.Println(strings.TrimSpace(string(output)))
			return nil
		}

		// Archive original MEMORY.md before Claude modifies anything
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

		fmt.Println("Consolidating memory with Claude...")

		// Invoke Claude — it reads/writes files directly
		var stderr strings.Builder
		claude := exec.Command("claude", "-p",
			"--output-format", "text",
			dreamPrompt)
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

		// Print Claude's summary
		summary := strings.TrimSpace(string(output))
		if summary != "" {
			fmt.Println(summary)
		}

		// Report results
		newLines := origLines
		if newData, err := os.ReadFile(memoryPath); err == nil {
			newLines = lineCount(string(newData))
		}
		topicCount := len(listTopicFiles(memDir))

		fmt.Printf("\nDream complete. MEMORY.md: %d → %d lines, %d topic files.\n", origLines, newLines, topicCount)
		return nil
	},
}

// listTopicFiles returns absolute paths of all files in memDir (excluding archive/).
func listTopicFiles(memDir string) []string {
	var files []string
	filepath.WalkDir(memDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(memDir, path)
		if strings.HasPrefix(rel, "archive/") || strings.HasPrefix(rel, "archive\\") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files
}

func lineCount(s string) int {
	return len(strings.Split(strings.TrimRight(s, "\n"), "\n"))
}

func init() {
	dreamCmd.Flags().Bool("dry-run", false, "Show what would change without applying")
}
