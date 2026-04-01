package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var migrateMemoryCmd = &cobra.Command{
	Use:   "migrate-memory [agents...]",
	Short: "Create memory/ directory for existing agents",
	Long: `Creates the memory/ topic directory alongside MEMORY.md for existing agent workspaces.

Examples:
  cubit migrate-memory noah alice   # specific agents
  cubit migrate-memory              # default agent from config`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agents := args
		if len(agents) == 0 {
			agents = []string{cfg.Agent}
		}

		for _, agent := range agents {
			if !isValidAgentName(agent) {
				fmt.Fprintf(os.Stderr, "%s: invalid agent name\n", agent)
				continue
			}
			agentDir := filepath.Join(cfg.Root, "agents-home", agent)
			if _, err := os.Stat(agentDir); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "%s: agent workspace not found at %s\n", agent, agentDir)
				continue
			}

			memDir := filepath.Join(agentDir, "memory")
			if _, err := os.Stat(memDir); err == nil {
				fmt.Printf("%s: memory/ already exists\n", agent)
				continue
			}

			if err := os.MkdirAll(memDir, 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "%s: failed to create memory/: %v\n", agent, err)
				continue
			}

			fmt.Printf("%s: created memory/ at %s\n", agent, memDir)
		}

		return nil
	},
}
