package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var requeueCmd = &cobra.Command{
	Use:   "requeue [id]",
	Short: "Return an active task to the queue",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid task ID: %s", args[0])
			}
			if err := q.RequeueByID(id); err != nil {
				return err
			}
			fmt.Printf("↩ %03d\n", id)
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
			return fmt.Errorf("specify task ID: cubit requeue <id>")
		}

		if err := q.RequeueByID(active[0].ID); err != nil {
			return err
		}
		fmt.Printf("↩ %03d: %s\n", active[0].ID, active[0].Title)
		return nil
	},
}
