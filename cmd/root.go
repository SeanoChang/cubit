package cmd

import (
	"fmt"
	"os"

	"github.com/SeanoChang/cubit/internal/config"
	"github.com/spf13/cobra"
)

var cfg *config.Config

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

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(archiveCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
