package agent

import (
	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
)

var (
	getCfg func() *config.Config
	getQ   func() *queue.Queue
)

// Register adds all agent state commands to root.
func Register(root *cobra.Command, cfgFn func() *config.Config, qFn func() *queue.Queue) {
	getCfg = cfgFn
	getQ = qFn

	root.AddCommand(statusCmd)
	root.AddCommand(briefCmd)
	root.AddCommand(refreshCmd)

	// cubit identity list|show|set
	identitySetCmd.Flags().StringP("file", "f", "", "Read content from file")
	identityCmd.AddCommand(identityListCmd)
	identityCmd.AddCommand(identityShowCmd)
	identityCmd.AddCommand(identitySetCmd)
	root.AddCommand(identityCmd)

	// cubit memory [show|append|edit]
	memoryCmd.AddCommand(memoryShowCmd)
	memoryCmd.AddCommand(memoryAppendCmd)
	memoryCmd.AddCommand(memoryEditCmd)
	root.AddCommand(memoryCmd)
}
