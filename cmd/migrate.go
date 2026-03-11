package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SeanoChang/cubit/internal/scaffold"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate v0.x workspace to v1.0 layout",
	Long:  "Moves agent data from ~/.ark/cubit/<agent>/ to ~/.ark/<agent>/ with new filesystem layout.",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := cfg.Agent
		home, _ := os.UserHomeDir()
		oldAgentDir := filepath.Join(home, ".ark", "cubit", agent)
		newAgentDir := cfg.AgentDir() // ~/.ark/<agent>

		// Check old layout exists
		if _, err := os.Stat(oldAgentDir); os.IsNotExist(err) {
			return fmt.Errorf("no v0.x workspace found at %s", oldAgentDir)
		}

		// Check new layout doesn't already exist
		if _, err := os.Stat(newAgentDir); err == nil {
			return fmt.Errorf("v1.0 workspace already exists at %s — nothing to migrate", newAgentDir)
		}

		fmt.Printf("Migrating %q: %s → %s\n\n", agent, oldAgentDir, newAgentDir)

		// Step 1: Create new workspace via scaffold
		created, err := scaffold.Init(cfg.Root, agent, false)
		if err != nil {
			return fmt.Errorf("creating v1.0 workspace: %w", err)
		}
		if !created {
			return fmt.Errorf("failed to create v1.0 workspace")
		}

		// Step 2: Migrate user data files
		migrations := []struct {
			src  string
			dst  string
			desc string
		}{
			{
				filepath.Join(oldAgentDir, "identity", "FLUCTLIGHT.md"),
				filepath.Join(newAgentDir, "FLUCTLIGHT.md"),
				"FLUCTLIGHT.md (identity)",
			},
			{
				filepath.Join(oldAgentDir, "GOALS.md"),
				filepath.Join(newAgentDir, "GOALS.md"),
				"GOALS.md",
			},
			{
				filepath.Join(oldAgentDir, "memory", "MEMORY.md"),
				filepath.Join(newAgentDir, "MEMORY.md"),
				"MEMORY.md",
			},
			{
				filepath.Join(oldAgentDir, "memory", "log.md"),
				filepath.Join(newAgentDir, "log.md"),
				"log.md",
			},
		}

		for _, m := range migrations {
			if err := copyFileIfExists(m.src, m.dst); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: %s — %v\n", m.desc, err)
			} else if fileExists(m.src) {
				fmt.Printf("  migrated %s\n", m.desc)
			}
		}

		// Step 3: Merge brief.md into MEMORY.md if it has content
		briefPath := filepath.Join(oldAgentDir, "memory", "brief.md")
		if briefData, err := os.ReadFile(briefPath); err == nil && len(briefData) > 0 {
			memoryPath := filepath.Join(newAgentDir, "MEMORY.md")
			f, err := os.OpenFile(memoryPath, os.O_APPEND|os.O_WRONLY, 0o644)
			if err == nil {
				f.WriteString("\n## Working Context (migrated from brief.md)\n\n")
				f.Write(briefData)
				f.Close()
				fmt.Println("  migrated brief.md → merged into MEMORY.md")
			}
		}

		// Step 4: Copy scratch/ contents (if any)
		oldScratch := filepath.Join(oldAgentDir, "scratch")
		newScratch := filepath.Join(newAgentDir, "scratch")
		if entries, err := os.ReadDir(oldScratch); err == nil {
			count := 0
			for _, entry := range entries {
				src := filepath.Join(oldScratch, entry.Name())
				dst := filepath.Join(newScratch, entry.Name())
				if entry.IsDir() {
					copyDir(src, dst)
				} else {
					copyFileIfExists(src, dst)
				}
				count++
			}
			if count > 0 {
				fmt.Printf("  migrated scratch/ (%d items)\n", count)
			}
		}

		// Step 5: Write new config at ~/.ark/config.yaml
		newConfigPath := filepath.Join(cfg.Root, "config.yaml")
		if _, err := os.Stat(newConfigPath); os.IsNotExist(err) {
			configContent := fmt.Sprintf("agent: %s\nroot: %s\n", agent, cfg.Root)
			os.WriteFile(newConfigPath, []byte(configContent), 0o644)
			fmt.Println("  wrote config.yaml")
		}

		// Step 6: Rename old directory as backup
		backupDir := oldAgentDir + ".v0-backup"
		if err := os.Rename(oldAgentDir, backupDir); err != nil {
			fmt.Fprintf(os.Stderr, "\n  warning: could not rename old workspace: %v\n", err)
			fmt.Fprintf(os.Stderr, "  old workspace remains at %s\n", oldAgentDir)
		} else {
			fmt.Printf("\n  Old workspace backed up to %s\n", backupDir)
		}

		fmt.Printf("\nMigration complete. Run 'cubit status' to verify.\n")
		return nil
	},
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFileIfExists(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return nil // source doesn't exist, skip silently
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFileIfExists(path, target)
	})
}
