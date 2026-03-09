package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshaling config: %w", err)
		}

		fmt.Print(string(out))
		return nil
	},
}
