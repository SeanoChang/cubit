package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cubit",
	Short: "Control plane for a single agent instance",
	Long:  "Cubit manages identity, sessions, tasks, and memory for an agent.",
}

// init registers the full command tree in one place.
// Individual commands are defined in their own files.
func init() {
	// cubit version
	rootCmd.AddCommand(versionCmd)

	// cubit config show
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
