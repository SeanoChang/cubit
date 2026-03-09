package cmd

import (
	"fmt"
	"os"

	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
)

// Shared state — loaded once via PersistentPreRunE.
var (
	cfg *config.Config
	q   *queue.Queue
)

var rootCmd = &cobra.Command{
	Use:   "cubit",
	Short: "Control plane for a single agent instance",
	Long:  "Cubit manages identity, sessions, tasks, and memory for an agent.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		q = queue.GetQueue(cfg.AgentDir())
		return nil
	},
}

// init registers the full command tree in one place.
// Individual commands are defined in their own files.
func init() {
	// cubit version
	rootCmd.AddCommand(versionCmd)

	// cubit init [agent_name] [--skip-onboard] [--import-identity FILE] [--force]
	initCmd.Flags().Bool("skip-onboard", false, "Skip interactive LLM onboarding")
	initCmd.Flags().String("import-identity", "", "Import an existing FLUCTLIGHT.md file")
	initCmd.Flags().Bool("force", false, "Re-run onboarding for an existing agent")
	rootCmd.AddCommand(initCmd)

	// cubit config show
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)

	// cubit todo "description" [--context "..."] [-f file.md]
	todoCmd.Flags().StringP("context", "c", "", "Inline context to append to task body")
	todoCmd.Flags().StringP("file", "f", "", "Read context from file")
	rootCmd.AddCommand(todoCmd)

	// cubit queue
	rootCmd.AddCommand(queueCmd)

	// cubit do
	rootCmd.AddCommand(doCmd)

	// cubit done ["summary"]
	rootCmd.AddCommand(doneCmd)

	// cubit requeue
	rootCmd.AddCommand(requeueCmd)

	// cubit log "observation"
	rootCmd.AddCommand(logCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
