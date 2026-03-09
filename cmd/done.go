package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var doneCmd = &cobra.Command{
	Use:   "done [id] [summary]",
	Short: "Complete an active task",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse optional task ID from first arg
		var taskID int
		var summary string
		if len(args) > 0 {
			if id, err := strconv.Atoi(args[0]); err == nil {
				taskID = id
				summary = strings.Join(args[1:], " ")
			} else {
				summary = strings.Join(args, " ")
			}
		}

		if taskID > 0 {
			if err := q.CompleteByID(taskID, summary); err != nil {
				return err
			}
			fmt.Printf("✓ %03d\n", taskID)
			return nil
		}

		active, err := q.Active()
		if err != nil {
			return err
		}
		if len(active) == 0 {
			return fmt.Errorf("no active task")
		}
		if len(active) > 1 {
			fmt.Println("Multiple active tasks:")
			for _, t := range active {
				fmt.Printf("  %03d: %s\n", t.ID, t.Title)
			}
			return fmt.Errorf("specify task ID: cubit done <id> [summary]")
		}

		if err := q.CompleteByID(active[0].ID, summary); err != nil {
			return err
		}
		fmt.Printf("✓ %03d: %s\n", active[0].ID, active[0].Title)
		return nil
	},
}
