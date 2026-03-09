package cmd

import (
	"fmt"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/SeanoChang/cubit/internal/claude"
	"github.com/spf13/cobra"
)

var promptCmd = &cobra.Command{
	Use:   `prompt "<message>"`,
	Short: "Single-shot prompt with brief injection and memory pass",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		injection := brief.Build(cfg.AgentDir())
		full := injection + "\n\n---\n\n" + args[0]

		result, err := claude.Prompt(full, cfg.Claude.Model)
		if err != nil {
			return err
		}

		fmt.Printf("\n%s\n", result)

		noMemory, _ := cmd.Flags().GetBool("no-memory")
		if !noMemory {
			if err := brief.RunMemoryPass(cfg.AgentDir(), result, cfg.Claude.MemoryModel); err != nil {
				fmt.Printf("warning: memory pass failed: %v\n", err)
			}
		}

		return nil
	},
}
