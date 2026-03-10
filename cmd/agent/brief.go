package agent

import (
	"fmt"
	"strings"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/spf13/cobra"
)

var briefCmd = &cobra.Command{
	Use:   "brief",
	Short: "Show brief injection sections and token estimates",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getCfg()

		sections := brief.Sections(cfg.AgentDir())

		total := 0
		for _, s := range sections {
			tokens := brief.EstimateTokens(s.Content)
			total += tokens
			fmt.Printf("  %-25s %s\n", s.Label, brief.FormatTokens(s.Content))
		}

		fmt.Println("  " + strings.Repeat("─", 40))
		fmt.Printf("  %-25s ~%d tokens\n", "Total", total)

		return nil
	},
}
