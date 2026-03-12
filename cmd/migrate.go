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
	Use:   "migrate [agents...]",
	Short: "Migrate workspaces to agents-home layout",
	Long:  "Moves agent data to ~/.ark/agents-home/<agent>/. Supports v0.x (~/.ark/cubit/<agent>/) and flat v1.0 (~/.ark/<agent>/) sources.\n\nExamples:\n  cubit migrate noah scout      # migrate specific agents\n  cubit migrate                  # migrate the default agent from config",
	RunE: func(cmd *cobra.Command, args []string) error {
		agents := args
		if len(agents) == 0 {
			agents = []string{cfg.Agent}
		}

		var errors []string
		for _, agent := range agents {
			if err := migrateAgent(agent); err != nil {
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

func migrateAgent(agent string) error {
	home, _ := os.UserHomeDir()
	newAgentDir := filepath.Join(cfg.Root, "agents-home", agent)

	// Check new layout doesn't already exist
	if _, err := os.Stat(newAgentDir); err == nil {
		return fmt.Errorf("workspace already exists at %s — nothing to migrate", newAgentDir)
	}

	// Detect source layout: flat v1.0 first, then v0.x
	flatDir := filepath.Join(cfg.Root, agent)
	oldV0Dir := filepath.Join(home, ".ark", "cubit", agent)

	// Check for flat v1.0 layout (has FLUCTLIGHT.md)
	if _, err := os.Stat(filepath.Join(flatDir, "FLUCTLIGHT.md")); err == nil {
		return migrateFlatLayout(agent, flatDir, newAgentDir)
	}

	if _, err := os.Stat(oldV0Dir); err == nil {
		return migrateV0Layout(agent, oldV0Dir, newAgentDir)
	}

	return fmt.Errorf("no workspace found at %s or %s — nothing to migrate", flatDir, oldV0Dir)
}

// migrateFlatLayout moves ~/.ark/<agent>/ to ~/.ark/agents-home/<agent>/.
func migrateFlatLayout(agent, srcDir, dstDir string) error {
	fmt.Printf("Migrating %q (flat → agents-home): %s → %s\n\n", agent, srcDir, dstDir)

	// Create agents-home/ parent
	agentsHome := filepath.Dir(dstDir)
	if err := os.MkdirAll(agentsHome, 0o755); err != nil {
		return fmt.Errorf("creating agents-home: %w", err)
	}

	// Move the directory
	if err := os.Rename(srcDir, dstDir); err != nil {
		return fmt.Errorf("moving workspace: %w", err)
	}

	fmt.Printf("  moved %s → %s\n", srcDir, dstDir)
	fmt.Printf("\nMigration complete for %q. Run 'cubit status' to verify.\n\n", agent)
	return nil
}

// migrateV0Layout scaffolds a new workspace and copies v0.x data into it.
func migrateV0Layout(agent, oldAgentDir, newAgentDir string) error {
	fmt.Printf("Migrating %q (v0.x → agents-home): %s → %s\n\n", agent, oldAgentDir, newAgentDir)

	// Step 1: Create new workspace via scaffold
	created, err := scaffold.Init(cfg.Root, agent, false)
	if err != nil {
		return fmt.Errorf("creating workspace: %w", err)
	}
	if !created {
		return fmt.Errorf("failed to create workspace")
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

	fmt.Printf("\nMigration complete for %q. Run 'cubit status' to verify.\n\n", agent)
	return nil
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
