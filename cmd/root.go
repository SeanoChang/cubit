package cmd

import (
	"fmt"
	"os"
	"time"

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
// Individual commands are defined in their own files.
func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("cubit %s (commit: %s, built: %s)\n", Version, Commit, Date))

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
	//   [--mode once|loop] [--depends-on 1,2] [--program file.md]
	//   [--goal "expr"] [--max-iterations N] [--branch name]
	todoCmd.Flags().StringP("context", "c", "", "Inline context to append to task body")
	todoCmd.Flags().StringP("file", "f", "", "Read context from file")
	todoCmd.Flags().String("mode", "once", "Execution mode: once or loop")
	todoCmd.Flags().IntSlice("depends-on", nil, "Comma-separated task IDs this task depends on")
	todoCmd.Flags().String("program", "", "Program file re-injected each loop iteration")
	todoCmd.Flags().String("goal", "", "Exit condition for loop mode (agent evaluates)")
	todoCmd.Flags().Int("max-iterations", 0, "Maximum loop iterations (0 = unlimited)")
	todoCmd.Flags().String("branch", "", "Git branch for this task (convention, not enforced)")
	todoCmd.Flags().String("model", "", "Claude model override for this task (empty = use config default)")
	rootCmd.AddCommand(todoCmd)

	// cubit queue
	rootCmd.AddCommand(queueCmd)

	// cubit do [--all]
	doCmd.Flags().Bool("all", false, "Pop all ready tasks at once")
	rootCmd.AddCommand(doCmd)

	// cubit done ["summary"]
	rootCmd.AddCommand(doneCmd)

	// cubit requeue
	rootCmd.AddCommand(requeueCmd)

	// cubit log "observation"
	rootCmd.AddCommand(logCmd)

	// cubit prompt "message" [--no-memory]
	promptCmd.Flags().Bool("no-memory", false, "Skip the post-session memory pass")
	rootCmd.AddCommand(promptCmd)

	// cubit brief
	rootCmd.AddCommand(briefCmd)

	// cubit run [--once] [--cooldown 30s] [--no-memory] [--max-parallel N]
	runCmd.Flags().Bool("once", false, "Execute one task then stop")
	runCmd.Flags().Duration("cooldown", 30*time.Second, "Wait duration between tasks")
	runCmd.Flags().Bool("no-memory", false, "Skip the post-session memory pass")
	runCmd.Flags().Int("max-parallel", 0, "Max concurrent tasks (0 = NumCPU*4)")
	rootCmd.AddCommand(runCmd)

	// cubit status
	rootCmd.AddCommand(statusCmd)

	// cubit refresh
	rootCmd.AddCommand(refreshCmd)

	// cubit graph [id] [--status s] [--mode m] [--ascii]
	graphCmd.Flags().String("status", "", "Filter by status: ready,waiting,done,active (comma-separated)")
	graphCmd.Flags().String("mode", "", "Filter by mode: once or loop")
	graphCmd.Flags().Bool("ascii", false, "Render subgraph as ASCII tree instead of Mermaid (only with task ID)")
	rootCmd.AddCommand(graphCmd)

	// cubit update
	rootCmd.AddCommand(updateCmd)

	// cubit identity list|show|set
	identitySetCmd.Flags().StringP("file", "f", "", "Read content from file")
	identityCmd.AddCommand(identityListCmd)
	identityCmd.AddCommand(identityShowCmd)
	identityCmd.AddCommand(identitySetCmd)
	rootCmd.AddCommand(identityCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
