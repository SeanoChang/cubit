package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var todoCmd = &cobra.Command{
	Use:   "todo <description>",
	Short: "Create a new task in the queue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, _ := cmd.Flags().GetString("context")
		file, _ := cmd.Flags().GetString("file")

		if file != "" {
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("reading context file: %w", err)
			}
			if ctx != "" {
				ctx += "\n\n"
			}
			ctx += string(data)
		}

		task, err := q.Create(args[0], ctx)
		if err != nil {
			return err
		}
		fmt.Printf("created task %03d: %s\n", task.ID, task.Title)
		return nil
	},
}
