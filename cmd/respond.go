package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var respondCmd = &cobra.Command{
	Use:   "respond <delegation-id> <drafts-dir>",
	Short: "Respond to a delegation with results",
	Long: `Sends a delegation-response directory-mail back to the delegating agent.
Sets type: delegation-response and delegation_id in frontmatter automatically.

The drafts-dir should contain mail.md (your summary) plus any attachment files.

Example:
  cubit respond del-20260402T1430-research-task mailbox/drafts/my-results/`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !agentExplicit {
			return fmt.Errorf("agent not specified — use --agent <name> or run from inside an agent directory")
		}

		delID := args[0]
		draftPath := args[1]
		if !filepath.IsAbs(draftPath) {
			draftPath = filepath.Join(cfg.AgentDir(), draftPath)
		}

		delegator, err := findDelegator(cfg.AgentDir(), delID)
		if err != nil {
			return fmt.Errorf("cannot find original delegation: %w", err)
		}

		mailPath := filepath.Join(draftPath, "mail.md")
		data, err := os.ReadFile(mailPath)
		if err != nil {
			return fmt.Errorf("reading mail.md: %w", err)
		}

		fm, body, err := parseFrontmatter(string(data))
		if err != nil {
			return fmt.Errorf("parsing frontmatter: %w", err)
		}

		owner := filepath.Base(cfg.AgentDir())

		fm["from"] = owner
		fm["to"] = delegator
		fm["type"] = "delegation-response"
		fm["delegation_id"] = delID
		fm["category"] = "priority"
		if fm["timestamp"] == "" {
			fm["timestamp"] = time.Now().Format(time.RFC3339)
		}
		if fm["subject"] == "" {
			fm["subject"] = fmt.Sprintf("Response: %s", delID)
		}

		finalContent := buildMessage(fm, body)
		if err := os.WriteFile(mailPath, []byte(finalContent), 0o644); err != nil {
			return fmt.Errorf("updating mail.md: %w", err)
		}

		if err := sendDirectoryMail(draftPath); err != nil {
			return err
		}

		fmt.Printf("Delegation response sent to %s for %s\n", delegator, delID)
		return nil
	},
}

// findDelegator searches inbox and read directories for the original delegation mail
// matching the given delegation_id and returns the 'from' field (the delegator).
func findDelegator(agentDir, delID string) (string, error) {
	searchDirs := []string{
		filepath.Join(agentDir, "mailbox", "inbox", "important"),
		filepath.Join(agentDir, "mailbox", "inbox", "priority"),
		filepath.Join(agentDir, "mailbox", "inbox", "all"),
		filepath.Join(agentDir, "mailbox", "read"),
	}

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			var mailFilePath string
			if e.IsDir() {
				mailFilePath = filepath.Join(dir, e.Name(), "mail.md")
			} else if strings.HasSuffix(e.Name(), ".md") {
				mailFilePath = filepath.Join(dir, e.Name())
			} else {
				continue
			}

			data, err := os.ReadFile(mailFilePath)
			if err != nil {
				continue
			}
			fm, _, err := parseFrontmatter(string(data))
			if err != nil {
				continue
			}
			if fm["delegation_id"] == delID && fm["type"] == "delegation" {
				return fm["from"], nil
			}
		}
	}
	return "", fmt.Errorf("no delegation mail found with id %q in inbox or read", delID)
}
