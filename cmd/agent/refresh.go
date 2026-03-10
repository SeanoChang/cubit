package agent

import (
	"context"
	"fmt"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Rewrite brief.md from scratch using recent journals and log",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getCfg()

		fmt.Println("Refreshing brief.md from journals + log...")

		if err := brief.RunRefresh(context.Background(), cfg.AgentDir(), cfg.Claude.MemoryRunnerOpts(), cfg.Claude.RefreshJournals); err != nil {
			return err
		}

		fmt.Println("Done. brief.md rewritten.")
		return nil
	},
}
