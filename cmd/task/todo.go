package task

import (
	"fmt"
	"os"

	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
)

var todoCmd = &cobra.Command{
	Use:   "todo <description>",
	Short: "Create a new task in the queue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := getQ()

		ctx, _ := cmd.Flags().GetString("context")
		file, _ := cmd.Flags().GetString("file")
		mode, _ := cmd.Flags().GetString("mode")
		model, _ := cmd.Flags().GetString("model")
		dependsOn, _ := cmd.Flags().GetIntSlice("depends-on")
		program, _ := cmd.Flags().GetString("program")
		goal, _ := cmd.Flags().GetString("goal")
		maxIter, _ := cmd.Flags().GetInt("max-iterations")
		branch, _ := cmd.Flags().GetString("branch")

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

		// IntSlice returns [0] when flag is unset; treat that as empty.
		if len(dependsOn) == 1 && dependsOn[0] == 0 {
			dependsOn = nil
		}

		if len(dependsOn) > 0 {
			if err := q.ValidateDependencies(q.NextID(), dependsOn); err != nil {
				return fmt.Errorf("dependency validation: %w", err)
			}
		}

		task, err := q.Create(args[0], queue.CreateOptions{
			Context:       ctx,
			Mode:          mode,
			Model:         model,
			DependsOn:     dependsOn,
			Program:       program,
			Goal:          goal,
			MaxIterations: maxIter,
			Branch:        branch,
		})
		if err != nil {
			return err
		}
		fmt.Printf("created task %03d: %s\n", task.ID, task.Title)
		return nil
	},
}
