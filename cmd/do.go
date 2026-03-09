package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var doCmd = &cobra.Command{
	Use:   "do",
	Short: "Pop the next ready task (or all with --all)",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")

		if all {
			tasks, err := q.PopAllReady()
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Println("No ready tasks.")
				return nil
			}
			for _, t := range tasks {
				fmt.Printf("▶ %03d: %s\n", t.ID, t.Title)
			}
			fmt.Printf("(%d tasks active)\n", len(tasks))
			return nil
		}

		task, err := q.PopReady()
		if err != nil {
			return err
		}
		fmt.Printf("▶ %03d: %s\n", task.ID, task.Title)
		return nil
	},
}
