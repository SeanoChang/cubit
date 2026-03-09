package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var doneCmd = &cobra.Command{
	Use:   "done [summary]",
	Short: "Complete the active task",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		summary := strings.Join(args, " ")

		active, err := q.Active()
		if err != nil {
			return err
		}
		if active == nil {
			return fmt.Errorf("no active task")
		}

		if err := q.Complete(summary); err != nil {
			return err
		}
		fmt.Printf("✓ %03d: %s\n", active.ID, active.Title)
		return nil
	},
}
