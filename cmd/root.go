package cmd

import (
	"fmt"
	"os"

	"github.com/SeanoChang/cubit/cmd/agent"
	"github.com/SeanoChang/cubit/cmd/exec"
	"github.com/SeanoChang/cubit/cmd/task"
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
	Use:     "cubit",
	Short:   "Control plane for a single agent instance",
	Long:    "Cubit manages identity, sessions, tasks, and memory for an agent.",
	Version: Version,
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
func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("cubit %s (commit: %s, built: %s)\n", Version, Commit, Date))

	getCfg := func() *config.Config { return cfg }
	getQ := func() *queue.Queue { return q }

	// ── Core ────────────────────────────────────────────────────

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)

	// cubit init [--skip-onboard] [--import-identity FILE] [--force]
	initCmd.Flags().Bool("skip-onboard", false, "Skip interactive LLM onboarding")
	initCmd.Flags().String("import-identity", "", "Import an existing FLUCTLIGHT.md file")
	initCmd.Flags().Bool("force", false, "Re-run onboarding for an existing agent")
	rootCmd.AddCommand(initCmd)

	// cubit config show
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)

	// ── Subpackages ─────────────────────────────────────────────

	task.Register(rootCmd, getCfg, getQ)
	exec.Register(rootCmd, getCfg, getQ)
	agent.Register(rootCmd, getCfg, getQ)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
