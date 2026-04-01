package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage agent memory topic files",
}

var memoryLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List memory topic files with line counts",
	RunE: func(cmd *cobra.Command, args []string) error {
		memDir := filepath.Join(cfg.AgentDir(), "memory")

		if _, err := os.Stat(memDir); os.IsNotExist(err) {
			fmt.Println("No memory/ directory. Run 'cubit migrate-memory' to create it.")
			return nil
		}

		var totalFiles int
		var totalLines int

		err := filepath.WalkDir(memDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(memDir, path)
			if strings.HasPrefix(rel, "archive/") || strings.HasPrefix(rel, "archive\\") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			lines := len(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))
			if len(data) == 0 {
				lines = 0
			}
			fmt.Printf("  %-35s %4d lines\n", rel, lines)
			totalFiles++
			totalLines += lines
			return nil
		})
		if err != nil {
			return err
		}

		if totalFiles == 0 {
			fmt.Println("  (empty)")
		} else {
			fmt.Printf("\n  %d files, %d lines total\n", totalFiles, totalLines)
		}
		return nil
	},
}

var memoryCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Show memory size stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		agentDir := cfg.AgentDir()

		// MEMORY.md stats
		memPath := filepath.Join(agentDir, "MEMORY.md")
		if data, err := os.ReadFile(memPath); err == nil {
			lines := len(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))
			words := len(strings.Fields(string(data)))
			tokens := int(float64(words) * 1.3)
			fmt.Printf("MEMORY.md:  %d lines, ~%d tokens (%d words)\n", lines, tokens, words)
		} else {
			fmt.Println("MEMORY.md:  (not found)")
		}

		// memory/ dir stats
		memDir := filepath.Join(agentDir, "memory")
		if _, err := os.Stat(memDir); os.IsNotExist(err) {
			fmt.Println("memory/:    (not created)")
			return nil
		}

		var fileCount int
		var totalLines int
		var totalBytes int64

		filepath.WalkDir(memDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(memDir, path)
			if strings.HasPrefix(rel, "archive/") || strings.HasPrefix(rel, "archive\\") {
				return nil
			}
			info, infoErr := d.Info()
			if infoErr != nil {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			lines := len(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))
			if len(data) == 0 {
				lines = 0
			}
			fileCount++
			totalLines += lines
			totalBytes += info.Size()
			return nil
		})

		fmt.Printf("memory/:    %d files, %d lines, %s\n", fileCount, totalLines, formatBytes(totalBytes))
		return nil
	},
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	return fmt.Sprintf("%.1f KB", float64(b)/1024)
}

func init() {
	memoryCmd.AddCommand(memoryLsCmd)
	memoryCmd.AddCommand(memoryCheckCmd)
}
