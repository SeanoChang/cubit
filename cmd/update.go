package cmd

import (
	"fmt"

	"github.com/SeanoChang/cubit/internal/updater"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update cubit to the latest release",
	Long:  "Downloads the latest release from GitHub and replaces the current binary.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("current: %s\nchecking for updates...\n", Version)

		newVersion, err := updater.Update(Version)
		if err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		if newVersion == "" {
			fmt.Println("already up-to-date.")
			return nil
		}

		fmt.Printf("updated: %s → %s\n", Version, newVersion)
		return nil
	},
}
