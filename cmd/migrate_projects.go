package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var migrateProjectsCmd = &cobra.Command{
	Use:   "migrate-projects [agents...]",
	Short: "Migrate workspaces from git-at-root to projects/ layout",
	Long: `Moves .git from the workspace root into projects/legacy/.
Creates projects/ directory. Removes workspace-level .gitignore.

Workspaces created by older versions of cubit init had the entire
agent directory as a git repo. This command transitions to the new
model where only individual projects inside projects/ are git repos.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agents := args
		if len(agents) == 0 {
			agents = []string{cfg.Agent}
		}

		var errors []string
		for _, agent := range agents {
			if err := migrateToProjects(agent); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", agent, err))
			}
		}

		if len(errors) > 0 {
			fmt.Fprintln(os.Stderr)
			for _, e := range errors {
				fmt.Fprintf(os.Stderr, "error: %s\n", e)
			}
			return fmt.Errorf("%d agent(s) failed to migrate", len(errors))
		}
		return nil
	},
}

func migrateToProjects(agent string) error {
	agentDir := filepath.Join(cfg.Root, "agents-home", agent)
	gitDir := filepath.Join(agentDir, ".git")

	// Check workspace exists
	if _, err := os.Stat(agentDir); err != nil {
		return fmt.Errorf("workspace not found at %s", agentDir)
	}

	// Check .git exists at workspace root
	if _, err := os.Stat(gitDir); err != nil {
		fmt.Printf("Agent %q: no .git at workspace root — already migrated or no git history.\n", agent)
		// Still ensure projects/ exists
		projDir := filepath.Join(agentDir, "projects")
		if _, err := os.Stat(projDir); err != nil {
			if mkErr := os.MkdirAll(projDir, 0o755); mkErr != nil {
				return fmt.Errorf("creating projects/: %w", mkErr)
			}
			fmt.Printf("  created projects/\n")
		}
		return nil
	}

	fmt.Printf("Migrating %q: removing git-at-root, creating projects/ layout\n\n", agent)

	// Check if there's meaningful history
	hasHistory := false
	gitCmd := exec.Command("git", "rev-list", "--count", "HEAD")
	gitCmd.Dir = agentDir
	if out, err := gitCmd.Output(); err == nil {
		count := strings.TrimSpace(string(out))
		if count != "0" {
			hasHistory = true
		}
	}

	// Create projects/
	projDir := filepath.Join(agentDir, "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		return fmt.Errorf("creating projects/: %w", err)
	}
	fmt.Println("  created projects/")

	if hasHistory {
		// Move .git into projects/legacy/
		legacyDir := filepath.Join(projDir, "legacy")
		if err := os.MkdirAll(legacyDir, 0o755); err != nil {
			return fmt.Errorf("creating projects/legacy/: %w", err)
		}

		legacyGit := filepath.Join(legacyDir, ".git")
		if err := os.Rename(gitDir, legacyGit); err != nil {
			return fmt.Errorf("moving .git to projects/legacy/: %w", err)
		}
		fmt.Println("  moved .git → projects/legacy/.git")

		// Copy non-OS work files into legacy project
		osFiles := map[string]bool{
			"FLUCTLIGHT.md": true, "PROGRAM.md": true, "GOALS.md": true,
			"MEMORY.md": true, "log.md": true, "DELIVER.md": true,
			"INBOX.md": true, ".claude": true, "scratch": true,
			"projects": true, ".gitignore": true,
		}

		workEntries, _ := os.ReadDir(agentDir)
		copied := 0
		for _, entry := range workEntries {
			if osFiles[entry.Name()] || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			src := filepath.Join(agentDir, entry.Name())
			dst := filepath.Join(legacyDir, entry.Name())
			var copyErr error
			if entry.IsDir() {
				copyErr = copyDir(src, dst)
			} else {
				copyErr = copyFileIfExists(src, dst)
			}
			if copyErr != nil {
				fmt.Fprintf(os.Stderr, "  warning: failed to copy %s: %v\n", entry.Name(), copyErr)
			} else {
				copied++
			}
		}
		if copied > 0 {
			fmt.Printf("  copied %d work file(s) to projects/legacy/\n", copied)
		}
	} else {
		// No meaningful history — just remove .git
		os.RemoveAll(gitDir)
		fmt.Println("  removed .git (no meaningful history)")
	}

	// Remove workspace .gitignore
	gitignorePath := filepath.Join(agentDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		os.Remove(gitignorePath)
		fmt.Println("  removed .gitignore")
	}

	// Verify workspace files intact
	critical := []string{"FLUCTLIGHT.md", "GOALS.md", "MEMORY.md", "PROGRAM.md", "log.md"}
	allPresent := true
	for _, f := range critical {
		if _, err := os.Stat(filepath.Join(agentDir, f)); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %s missing\n", f)
			allPresent = false
		}
	}
	if allPresent {
		fmt.Println("  workspace files verified")
	}

	fmt.Printf("\nMigration complete for %q. Run 'cubit status' to verify.\n\n", agent)
	return nil
}
