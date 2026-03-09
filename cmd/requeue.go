package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var requeueCmd = &cobra.Command{
	Use:   "requeue",
	Short: "Return the active task to the queue",
	RunE: func(cmd *cobra.Command, args []string) error {
		active, err := q.Active()
		if err != nil {
			return err
		}
		if active == nil {
			return fmt.Errorf("no active task")
		}

		if err := q.Requeue(); err != nil {
			return err
		}
		fmt.Printf("↩ %03d: %s\n", active.ID, active.Title)
		return nil
	},
}
