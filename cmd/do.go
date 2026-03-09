package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var doCmd = &cobra.Command{
	Use:   "do",
	Short: "Pop the next task from the queue and make it active",
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := q.Pop()
		if err != nil {
			return err
		}
		fmt.Printf("▶ %03d: %s\n", task.ID, task.Title)
		return nil
	},
}
