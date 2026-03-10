package mcp

import (
	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
)

var (
	getCfg func() *config.Config
	getQ   func() *queue.Queue
)

// Register adds the cubit mcp command to root.
func Register(root *cobra.Command, cfgFn func() *config.Config, qFn func() *queue.Queue) {
	getCfg = cfgFn
	getQ = qFn
	root.AddCommand(mcpCmd)
}
