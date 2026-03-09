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
		for _, t := range active {
			fmt.Printf("  %03d  ← doing  %s\n", t.ID, t.Title)
		}

		tasks, err := q.List()
		if err != nil {
			return err
		}
		if len(tasks) == 0 && len(active) == 0 {
			fmt.Println("queue is empty")
			return nil
		}
		for _, t := range tasks {
			fmt.Printf("  %03d  %s\n", t.ID, t.Title)
		}
		return nil
	},
}
