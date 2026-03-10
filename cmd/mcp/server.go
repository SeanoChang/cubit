package mcp

import (
	"os"

	internalmcp "github.com/SeanoChang/cubit/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server on stdio",
	Long:  "Starts a Model Context Protocol server over stdin/stdout for Claude Code integration.",
	RunE: func(cmd *cobra.Command, args []string) error {
		srv := internalmcp.NewServer(getCfg(), getQ())
		return srv.Run(os.Stdin, os.Stdout)
	},
}
