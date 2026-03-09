package cmd

import (
	"fmt"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show agent status overview",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Agent:   %s\n", cfg.Agent)

		// Active task
		active, err := q.Active()
		if err != nil {
			return err
		}
		if active != nil {
			fmt.Printf("Active:  %03d — %s\n", active.ID, active.Title)
		} else {
			fmt.Println("Active:  (none)")
		}

		// Queue depth
		tasks, err := q.List()
		if err != nil {
			return err
		}
		fmt.Printf("Queue:   %d pending\n", len(tasks))

		// Brief and injection sizes
		sections := brief.Sections(cfg.AgentDir())
		total := 0
		var briefContent string
		for _, s := range sections {
			total += brief.EstimateTokens(s.Content)
			if s.Label == "Brief" {
				briefContent = s.Content
			}
		}

		fmt.Printf("Brief:   %s\n", brief.FormatTokens(briefContent))
		fmt.Printf("Inject:  ~%d tokens total\n", total)

		return nil
	},
}
