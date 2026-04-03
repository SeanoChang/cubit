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

var delegateCmd = &cobra.Command{
	Use:   "delegate <drafts-dir>",
	Short: "Delegate work to other agents with tracking",
	Long: `Creates a tracked delegation, sends directory-mail to each target agent,
and writes a delegation tracker to mailbox/delegations/active/.

Use --to and --task flags (repeatable) to specify targets and their tasks.
Use --on-complete to describe what should happen when all results are in.

Example:
  cubit delegate \
    --to agent2 --task "Analyze pricing pages" \
    --to agent3 --task "Review financials" \
    --on-complete "Synthesize into executive summary" \
    mailbox/drafts/research-context/`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !agentExplicit {
			return fmt.Errorf("agent not specified — use --agent <name> or run from inside an agent directory")
		}

		toFlags, _ := cmd.Flags().GetStringArray("to")
		taskFlags, _ := cmd.Flags().GetStringArray("task")
		onComplete, _ := cmd.Flags().GetString("on-complete")

		if len(toFlags) == 0 {
			return fmt.Errorf("at least one --to flag is required")
		}
		if len(toFlags) != len(taskFlags) {
			return fmt.Errorf("each --to must have a matching --task (got %d --to, %d --task)", len(toFlags), len(taskFlags))
		}

		draftPath := args[0]
		if !filepath.IsAbs(draftPath) {
			draftPath = filepath.Join(cfg.AgentDir(), draftPath)
		}

		// Verify draft directory exists and has mail.md
		mailPath := filepath.Join(draftPath, "mail.md")
		if _, err := os.Stat(mailPath); err != nil {
			return fmt.Errorf("draft directory must contain mail.md: %w", err)
		}

		// Read shared context from mail.md
		data, err := os.ReadFile(mailPath)
		if err != nil {
			return fmt.Errorf("reading mail.md: %w", err)
		}
		_, sharedBody, err := parseFrontmatter(string(data))
		if err != nil {
			return fmt.Errorf("parsing mail.md frontmatter: %w", err)
		}

		// Generate delegation ID
		slug := slugify(filepath.Base(draftPath))
		delID := generateDelegationID(slug)
		owner := filepath.Base(cfg.AgentDir())
		now := time.Now()

		// Create delegation tracker
		var subTasks []delegation.SubTask
		for i, to := range toFlags {
			if !isValidAgentName(to) {
				return fmt.Errorf("invalid agent name: %q", to)
			}
			subTasks = append(subTasks, delegation.SubTask{
				To:       to,
				Task:     taskFlags[i],
				Status:   delegation.SubStatusPending,
				Attempts: 1,
			})
		}

		del := &delegation.Delegation{
			ID:          delID,
			Created:     now.Format(time.RFC3339),
			Owner:       owner,
			GoalContext: fmt.Sprintf("Delegation from %s", owner),
			OnComplete:  onComplete,
			Status:      delegation.StatusPending,
			SubTasks:    subTasks,
		}

		// Write tracker
		activeDir := filepath.Join(cfg.AgentDir(), "mailbox", "delegations", "active")
		if err := os.MkdirAll(activeDir, 0o755); err != nil {
			return fmt.Errorf("creating delegations dir: %w", err)
		}
		if err := delegation.Write(activeDir, del); err != nil {
			return fmt.Errorf("writing delegation tracker: %w", err)
		}

		// Send directory-mail to each target
		for i, to := range toFlags {
			task := taskFlags[i]
			targetDraft := filepath.Join(cfg.AgentDir(), "mailbox", "drafts", fmt.Sprintf(".del-%s-%s", delID, to))
			if err := copyDir(draftPath, targetDraft); err != nil {
				return fmt.Errorf("copying draft for %s: %w", to, err)
			}

			body := fmt.Sprintf("**Your task:** %s\n\n%s", task, sharedBody)
			targetMailContent := buildMessage(map[string]string{
				"from":          owner,
				"to":            to,
				"subject":       fmt.Sprintf("Delegation: %s", task),
				"category":      "priority",
				"type":          "delegation",
				"delegation_id": delID,
				"timestamp":     now.Format(time.RFC3339),
			}, body)
			if err := os.WriteFile(filepath.Join(targetDraft, "mail.md"), []byte(targetMailContent), 0o644); err != nil {
				return fmt.Errorf("writing target mail.md: %w", err)
			}

			if err := sendDirectoryMail(targetDraft); err != nil {
				return fmt.Errorf("sending to %s: %w", to, err)
			}

			// Record dispatched mail name in tracker
			ts := now.Format("2006-01-02T15-04-05")
			taskSlug := slugify(fmt.Sprintf("Delegation: %s", task))
			del.SubTasks[i].DispatchedMail = fmt.Sprintf("%s-%s-%s/", ts, owner, taskSlug)
		}

		// Update tracker with dispatched mail paths
		if err := delegation.Write(activeDir, del); err != nil {
			return fmt.Errorf("updating delegation tracker: %w", err)
		}

		fmt.Printf("Delegation created: %s\n", delID)
		fmt.Printf("Targets: %s\n", strings.Join(toFlags, ", "))
		fmt.Println("Write to DELIVER.md to notify the user of this delegation.")
		return nil
	},
}

func generateDelegationID(slug string) string {
	ts := time.Now().Format("20060102T1504")
	if slug == "" {
		slug = "delegation"
	}
	return fmt.Sprintf("del-%s-%s", ts, slug)
}

func init() {
	delegateCmd.Flags().StringArray("to", nil, "Target agent name (repeatable)")
	delegateCmd.Flags().StringArray("task", nil, "Task for corresponding --to agent (repeatable)")
	delegateCmd.Flags().String("on-complete", "", "Instruction for when all sub-tasks complete")
}
