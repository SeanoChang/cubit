package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive scratch + log to nark, clean scratch",
	RunE: func(cmd *cobra.Command, args []string) error {
		agentDir := cfg.AgentDir()
		scratchDir := filepath.Join(agentDir, "scratch")

		// Collect log.md
		var content strings.Builder
		content.WriteString(fmt.Sprintf("# Archive — %s\n\n", cfg.Agent))

		logPath := filepath.Join(agentDir, "log.md")
		logData, logErr := os.ReadFile(logPath)
		hasLog := logErr == nil && len(logData) > 0
		if hasLog {
			content.WriteString("## log.md\n\n")
			content.WriteString(string(logData))
			content.WriteString("\n\n")
		}

		// Walk scratch/ recursively to collect all files (including subdirs)
		fileCount := 0
		filepath.WalkDir(scratchDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() == ".gitkeep" {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			relPath, _ := filepath.Rel(scratchDir, path)
			content.WriteString(fmt.Sprintf("## scratch/%s\n\n%s\n\n", relPath, string(data)))
			fileCount++
			return nil
		})

		if fileCount == 0 && !hasLog {
			fmt.Println("Nothing to archive.")
			return nil
		}

		// Try nark write — abort cleanup if nark fails
		date := time.Now().Format("2006-01-02")
		title := fmt.Sprintf("cubit archive %s %s", cfg.Agent, date)

		narkCmd := exec.Command("nark", "write", "--title", title)
		narkCmd.Stdin = strings.NewReader(content.String())
		output, err := narkCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("nark write failed: %v\nscratch/ not cleaned — archive manually or install nark", err)
		}

		narkID := strings.TrimSpace(string(output))
		fmt.Printf("Archived log + %d scratch files to nark: %s\n", fileCount, narkID)

		// Append log entry
		logEntry := fmt.Sprintf("\n[%s] Archived log + %d scratch files to nark: %s\n", date, fileCount, narkID)
		f, appendErr := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if appendErr == nil {
			f.WriteString(logEntry)
			f.Close()
		}

		// Clean scratch/ (keep .gitkeep, remove everything else)
		entries, _ := os.ReadDir(scratchDir)
		for _, entry := range entries {
			if entry.Name() == ".gitkeep" {
				continue
			}
			path := filepath.Join(scratchDir, entry.Name())
			if entry.IsDir() {
				os.RemoveAll(path)
			} else {
				os.Remove(path)
			}
		}
		fmt.Println("Cleaned scratch/")

		return nil
	},
}
