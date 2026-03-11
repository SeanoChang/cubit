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

		root := cfg.Root
		agentDir := filepath.Join(root, agent)
		force, _ := cmd.Flags().GetBool("force")

		created, err := scaffold.Init(root, agent, force)
		if err != nil {
			return fmt.Errorf("initializing agent: %w", err)
		}

		if !created {
			fmt.Printf("Agent %q already exists at %s (use --force to re-initialize)\n", agent, agentDir)
			return nil
		}

		if force {
			fmt.Printf("Re-initialized agent %q at %s\n", agent, agentDir)
		} else {
			fmt.Printf("Initialized agent %q at %s\n", agent, agentDir)
		}

		// --import-identity FILE: copy an existing FLUCTLIGHT.md
		importPath, _ := cmd.Flags().GetString("import-identity")
		if importPath != "" {
			data, err := os.ReadFile(importPath)
			if err != nil {
				return fmt.Errorf("reading identity file: %w", err)
			}
			dest := filepath.Join(agentDir, "FLUCTLIGHT.md")
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return err
			}
			fmt.Printf("  imported %s → FLUCTLIGHT.md\n", importPath)
		}

		return nil
	},
}

func init() {
	initCmd.Flags().String("import-identity", "", "Import an existing FLUCTLIGHT.md file")
	initCmd.Flags().Bool("force", false, "Re-initialize an existing agent workspace")
}
