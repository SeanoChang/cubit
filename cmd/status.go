package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SeanoChang/cubit/internal/project"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show agent workspace status",
	RunE: func(cmd *cobra.Command, args []string) error {
		agentDir := cfg.AgentDir()
		fmt.Printf("Agent: %s\n", cfg.Agent)
		fmt.Printf("Path:  %s\n\n", agentDir)

		// GOALS.md
		goalsPath := filepath.Join(agentDir, "GOALS.md")
		goals, err := os.ReadFile(goalsPath)
		if err != nil {
			fmt.Println("Goals: (not found)")
		} else {
			content := strings.TrimSpace(string(goals))
			if content == "" || content == "# Goals" {
				fmt.Println("Goals: (none)")
			} else {
				fmt.Printf("Goals:\n%s\n", content)
			}
		}
		fmt.Println()

		// MEMORY.md token estimate
		memoryPath := filepath.Join(agentDir, "MEMORY.md")
		memory, err := os.ReadFile(memoryPath)
		if err != nil {
			fmt.Println("Memory: (not found)")
		} else {
			words := len(strings.Fields(string(memory)))
			tokens := int(float64(words) * 1.3)
			fmt.Printf("Memory: ~%d tokens (%d words)\n", tokens, words)
		}

		// memory/ topic files
		memDir := filepath.Join(agentDir, "memory")
		var topicFiles int
		filepath.WalkDir(memDir, func(_ string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				topicFiles++
			}
			return nil
		})
		if topicFiles > 0 {
			fmt.Printf("Topics: %d files in memory/\n", topicFiles)
		}

		// Projects
		projects, projErr := project.List(agentDir)
		if projErr == nil && len(projects) > 0 {
			fmt.Printf("\nProjects (%d):\n", len(projects))
			for _, p := range projects {
				age := project.FormatAge(p.LastCommit)
				extra := ""
				if p.HasEval {
					extra = " [EVAL]"
				}
				fmt.Printf("  %-25s %3d commits, last: %-8s%s\n", p.Name, p.CommitCount, age, extra)
			}
		}
		fmt.Println()

		// log.md tail
		logPath := filepath.Join(agentDir, "log.md")
		logData, err := os.ReadFile(logPath)
		if err != nil {
			fmt.Println("Log:    (not found)")
		} else {
			lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
			n := 5
			if len(lines) < n {
				n = len(lines)
			}
			tail := lines[len(lines)-n:]
			fmt.Printf("Log (last %d lines):\n", n)
			for _, line := range tail {
				fmt.Printf("  %s\n", line)
			}
		}

		return nil
	},
}
