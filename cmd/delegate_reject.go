package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SeanoChang/cubit/internal/delegation"
	"github.com/spf13/cobra"
)

var delegateRejectCmd = &cobra.Command{
	Use:   "reject <delegation-id>",
	Short: "Reject a sub-task and re-dispatch",
	Long: `Marks a sub-task as rejected, increments attempt count, and sends a new
delegation mail to the target agent with the rejection reason.

Example:
  cubit delegate reject del-20260402T1430-research-task \
    --sub-task agent2 --reason "Missing EU pricing data"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !agentExplicit {
			return fmt.Errorf("agent not specified")
		}

		delID := args[0]
		subTaskAgent, _ := cmd.Flags().GetString("sub-task")
		reason, _ := cmd.Flags().GetString("reason")

		if subTaskAgent == "" {
			return fmt.Errorf("--sub-task is required")
		}
		if reason == "" {
			return fmt.Errorf("--reason is required")
		}

		activeDir := filepath.Join(cfg.AgentDir(), "mailbox", "delegations", "active")
		delDir := filepath.Join(activeDir, delID)
		jsonPath := filepath.Join(delDir, "delegation.json")

		d, err := delegation.Read(jsonPath)
		if err != nil {
			return fmt.Errorf("delegation not found: %w", err)
		}

		found := false
		var subTask *delegation.SubTask
		for i := range d.SubTasks {
			if d.SubTasks[i].To == subTaskAgent {
				if d.SubTasks[i].Status != delegation.SubStatusComplete {
					return fmt.Errorf("cannot reject sub-task for %s: status is %q (must be %q)", subTaskAgent, d.SubTasks[i].Status, delegation.SubStatusComplete)
				}
				d.SubTasks[i].Status = delegation.SubStatusRejected
				d.SubTasks[i].Attempts++
				d.SubTasks[i].ResponseMail = ""
				subTask = &d.SubTasks[i]
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no sub-task for agent %q in delegation %s", subTaskAgent, delID)
		}

		d.Recalculate()

		if err := delegation.Write(activeDir, d); err != nil {
			return fmt.Errorf("updating tracker: %w", err)
		}

		// Send rejection mail
		owner := filepath.Base(cfg.AgentDir())
		now := time.Now()
		draftDir := filepath.Join(cfg.AgentDir(), "mailbox", "drafts", fmt.Sprintf(".reject-%s-%s", delID, subTaskAgent))
		if err := os.MkdirAll(draftDir, 0o755); err != nil {
			return fmt.Errorf("creating rejection draft dir: %w", err)
		}

		body := fmt.Sprintf("**Revision requested (attempt %d)**\n\n**Reason:** %s\n\n**Original task:** %s",
			subTask.Attempts, reason, subTask.Task)
		mailContent := buildMessage(map[string]string{
			"from":          owner,
			"to":            subTaskAgent,
			"subject":       fmt.Sprintf("Revision: %s", subTask.Task),
			"category":      "priority",
			"type":          "delegation",
			"delegation_id": delID,
			"attempt":       fmt.Sprintf("%d", subTask.Attempts),
			"timestamp":     now.Format(time.RFC3339),
		}, body)
		if err := os.WriteFile(filepath.Join(draftDir, "mail.md"), []byte(mailContent), 0o644); err != nil {
			return fmt.Errorf("writing rejection mail: %w", err)
		}

		if err := sendDirectoryMail(draftDir); err != nil {
			return fmt.Errorf("sending rejection: %w", err)
		}

		fmt.Printf("Rejected %s's work on %s (attempt %d). Revision sent.\n", subTaskAgent, delID, subTask.Attempts)
		return nil
	},
}

func init() {
	delegateRejectCmd.Flags().String("sub-task", "", "Agent name of the sub-task to reject")
	delegateRejectCmd.Flags().String("reason", "", "Reason for rejection")
	delegateCmd.AddCommand(delegateRejectCmd)
}
