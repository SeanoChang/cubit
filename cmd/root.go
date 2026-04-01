package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SeanoChang/cubit/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfg            *config.Config
	agentExplicit  bool // true if agent was resolved via --agent flag or CWD detection
)

var rootCmd = &cobra.Command{
	Use:     "cubit",
	Short:   "Filesystem CLI for agent workspaces",
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}

		// --agent flag overrides config
		agentFlag, _ := cmd.Root().PersistentFlags().GetString("agent")
		if agentFlag != "" {
			cfg.Agent = agentFlag
			agentExplicit = true
		} else if detected := detectAgentFromCWD(cfg.Root); detected != "" {
			cfg.Agent = detected
			agentExplicit = true
		}

		// Warn if old layouts detected
		if cmd.Name() != "migrate" {
			if cfg.IsLegacyLayout() {
				fmt.Fprintf(os.Stderr, "Warning: Legacy v0.x workspace detected at %s\n", cfg.LegacyAgentDir())
				fmt.Fprintf(os.Stderr, "  Run 'cubit migrate' to upgrade.\n\n")
			} else if cfg.IsFlatLayout() {
				fmt.Fprintf(os.Stderr, "Warning: Flat v1.0 workspace detected at %s\n", cfg.FlatAgentDir())
				fmt.Fprintf(os.Stderr, "  Run 'cubit migrate' to move to agents-home layout.\n\n")
			}
		}

		return nil
	},
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("cubit %s (commit: %s, built: %s)\n", Version, Commit, Date))
	rootCmd.PersistentFlags().String("agent", "", "Target agent name (auto-detected from CWD if inside agents-home/)")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(migrateProjectsCmd)
	rootCmd.AddCommand(goalCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(migrateMemoryCmd)
	rootCmd.AddCommand(dreamCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(migrateMailboxCmd)
}

// detectAgentFromCWD checks if the current directory is inside agents-home/<agent>/
// and returns the agent name if so.
func detectAgentFromCWD(root string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	agentsHome := filepath.Join(root, "agents-home")
	rel, err := filepath.Rel(agentsHome, cwd)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return ""
	}
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
