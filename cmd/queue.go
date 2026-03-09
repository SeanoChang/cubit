package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "List pending tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		active, err := q.Active()
		if err != nil {
			return err
		}
		if active != nil {
			fmt.Printf("  %03d  ← doing  %s\n", active.ID, active.Title)
		}

		tasks, err := q.List()
		if err != nil {
			return err
		}
		if len(tasks) == 0 && active == nil {
			fmt.Println("queue is empty")
			return nil
		}
		for _, t := range tasks {
			fmt.Printf("  %03d  %s\n", t.ID, t.Title)
		}
		return nil
	},
}
