package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SeanoChang/cubit/internal/delegation"
	"github.com/spf13/cobra"
)

var delegateStatusCmd = &cobra.Command{
	Use:   "status [delegation-id]",
	Short: "Show delegation status",
	Long:  "Lists all active delegations, or shows detail for a specific delegation ID.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !agentExplicit {
			return fmt.Errorf("agent not specified")
		}

		activeDir := filepath.Join(cfg.AgentDir(), "mailbox", "delegations", "active")

		if len(args) == 1 {
			return showDelegation(activeDir, args[0])
		}
		return listDelegations(activeDir)
	},
}

func listDelegations(activeDir string) error {
	entries, err := os.ReadDir(activeDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No active delegations.")
			return nil
		}
		return err
	}

	found := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		jsonPath := filepath.Join(activeDir, e.Name(), "delegation.json")
		d, err := delegation.Read(jsonPath)
		if err != nil {
			continue
		}
		found = true
		complete := 0
		for _, st := range d.SubTasks {
			if st.Status == delegation.SubStatusComplete {
				complete++
			}
		}
		targets := make([]string, len(d.SubTasks))
		for i, st := range d.SubTasks {
			targets[i] = st.To
		}
		fmt.Printf("%s [%s] → %s (%d/%d complete)\n", d.ID, d.Status, strings.Join(targets, ", "), complete, len(d.SubTasks))
	}

	if !found {
		fmt.Println("No active delegations.")
	}
	return nil
}

func showDelegation(activeDir, id string) error {
	jsonPath := filepath.Join(activeDir, id, "delegation.json")
	d, err := delegation.Read(jsonPath)
	if err != nil {
		return fmt.Errorf("delegation not found: %w", err)
	}

	fmt.Printf("%s [%s]\n", d.ID, d.Status)
	fmt.Printf("Created: %s\n", d.Created)
	if d.OnComplete != "" {
		fmt.Printf("On complete: %s\n", d.OnComplete)
	}
	fmt.Println()

	now := time.Now()
	for _, st := range d.SubTasks {
		age := ""
		if created, err := time.Parse(time.RFC3339, d.Created); err == nil {
			age = fmt.Sprintf(", dispatched %s ago", now.Sub(created).Round(time.Minute))
		}
		fmt.Printf("  %s: %s (attempt %d%s)\n", st.To, st.Status, st.Attempts, age)
		fmt.Printf("    Task: %s\n", st.Task)
	}
	return nil
}

func init() {
	delegateCmd.AddCommand(delegateStatusCmd)
}
