package task

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log <observation>",
	Short: "Append an observation to the log",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := getQ()
		note := strings.Join(args, " ")

		if err := q.Log(note); err != nil {
			return err
		}
		fmt.Println("logged")
		return nil
	},
}
