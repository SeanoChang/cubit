package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SeanoChang/cubit/internal/scaffold"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [agent_name]",
	Short: "Scaffold a new agent workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var agent string
		if len(args) > 0 {
			agent = args[0]
		} else {
			fmt.Print("Agent name: ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				agent = strings.TrimSpace(scanner.Text())
			}
			if agent == "" {
				return fmt.Errorf("agent name is required")
			}
		}

		agentDir := filepath.Join(cfg.Root, agent)
		force, _ := cmd.Flags().GetBool("force")

		created, err := scaffold.Init(cfg.Root, agent)
		if err != nil {
			return fmt.Errorf("initializing agent: %w", err)
		}

		if !created && !force {
			fmt.Printf("Agent %q already initialized at %s\n", agent, agentDir)
			return nil
		}

		if created {
			fmt.Printf("Initialized agent %q at %s\n", agent, agentDir)
		} else {
			fmt.Printf("Re-running setup for agent %q at %s\n", agent, agentDir)
		}

		// --import-identity FILE: copy an existing FLUCTLIGHT.md
		importPath, _ := cmd.Flags().GetString("import-identity")
		if importPath != "" {
			data, err := os.ReadFile(importPath)
			if err != nil {
				return fmt.Errorf("reading identity file: %w", err)
			}
			dest := filepath.Join(agentDir, "identity", "FLUCTLIGHT.md")
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return err
			}
			fmt.Printf("  imported %s → %s\n", importPath, dest)
			return nil
		}

		// --skip-onboard: scaffold only, no interactive setup
		skipOnboard, _ := cmd.Flags().GetBool("skip-onboard")
		if !skipOnboard {
			if err := scaffold.RunSetup(agentDir, agent); err != nil {
				return fmt.Errorf("setup: %w", err)
			}
		}

		return nil
	},
}
