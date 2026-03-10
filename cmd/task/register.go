package task

import (
	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
)

var (
	getCfg func() *config.Config
	getQ   func() *queue.Queue
)

// Register adds all task management commands to root.
func Register(root *cobra.Command, cfgFn func() *config.Config, qFn func() *queue.Queue) {
	getCfg = cfgFn
	getQ = qFn

	// cubit todo "description" [flags]
	todoCmd.Flags().StringP("context", "c", "", "Inline context to append to task body")
	todoCmd.Flags().StringP("file", "f", "", "Read context from file")
	todoCmd.Flags().String("mode", "once", "Execution mode: once or loop")
	todoCmd.Flags().IntSlice("depends-on", nil, "Comma-separated task IDs this task depends on")
	todoCmd.Flags().String("program", "", "Program file re-injected each loop iteration")
	todoCmd.Flags().String("goal", "", "Exit condition for loop mode (agent evaluates)")
	todoCmd.Flags().Int("max-iterations", 0, "Maximum loop iterations (0 = unlimited)")
	todoCmd.Flags().String("branch", "", "Git branch for this task (convention, not enforced)")
	todoCmd.Flags().String("model", "", "Claude model override for this task (empty = use config default)")
	root.AddCommand(todoCmd)

	root.AddCommand(queueCmd)

	// cubit do [--all]
	doCmd.Flags().Bool("all", false, "Pop all ready tasks at once")
	root.AddCommand(doCmd)

	root.AddCommand(doneCmd)
	root.AddCommand(requeueCmd)
	root.AddCommand(logCmd)

	// cubit graph [id] [--status s] [--mode m] [--ascii]
	graphCmd.Flags().String("status", "", "Filter by status: ready,waiting,done,active (comma-separated)")
	graphCmd.Flags().String("mode", "", "Filter by mode: once or loop")
	graphCmd.Flags().Bool("ascii", false, "Render subgraph as ASCII tree instead of Mermaid (only with task ID)")
	root.AddCommand(graphCmd)
}
