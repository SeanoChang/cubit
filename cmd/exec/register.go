package exec

import (
	"time"

	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
)

var (
	getCfg func() *config.Config
	getQ   func() *queue.Queue
)

// Register adds all execution commands to root.
func Register(root *cobra.Command, cfgFn func() *config.Config, qFn func() *queue.Queue) {
	getCfg = cfgFn
	getQ = qFn

	// cubit prompt "message" [--no-memory]
	promptCmd.Flags().Bool("no-memory", false, "Skip the post-session memory pass")
	root.AddCommand(promptCmd)

	// cubit run [--once] [--cooldown 30s] [--no-memory] [--max-parallel N]
	runCmd.Flags().Bool("once", false, "Execute one task then stop")
	runCmd.Flags().Duration("cooldown", 30*time.Second, "Wait duration between tasks")
	runCmd.Flags().Bool("no-memory", false, "Skip the post-session memory pass")
	runCmd.Flags().Int("max-parallel", 0, "Max concurrent tasks (0 = NumCPU*4)")
	root.AddCommand(runCmd)
}
