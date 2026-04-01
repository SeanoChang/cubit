package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var migrateMailboxCmd = &cobra.Command{
	Use:   "migrate-mailbox [agents...]",
	Short: "Create mailbox/ directory tree for existing agents",
	Long: `Creates the mailbox directory structure and migrates INBOX.md content
into mailbox/inbox/all/ as a system message.

Examples:
  cubit migrate-mailbox noah alice   # specific agents
  cubit migrate-mailbox              # default agent from config`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agents := args
		if len(agents) == 0 {
			agents = []string{cfg.Agent}
		}

		for _, agent := range agents {
			if !isValidAgentName(agent) {
				fmt.Fprintf(os.Stderr, "%s: invalid agent name\n", agent)
				continue
			}
			agentDir := filepath.Join(cfg.Root, "agents-home", agent)
			if _, err := os.Stat(agentDir); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "%s: agent workspace not found at %s\n", agent, agentDir)
				continue
			}

			mailboxDir := filepath.Join(agentDir, "mailbox")
			if _, err := os.Stat(mailboxDir); err == nil {
				fmt.Printf("%s: mailbox/ already exists\n", agent)
				continue
			}

			// Create mailbox tree
			dirs := []string{
				filepath.Join(mailboxDir, "inbox", "important"),
				filepath.Join(mailboxDir, "inbox", "priority"),
				filepath.Join(mailboxDir, "inbox", "all"),
				filepath.Join(mailboxDir, "starred"),
				filepath.Join(mailboxDir, "drafts"),
				filepath.Join(mailboxDir, "sent"),
				filepath.Join(mailboxDir, "read"),
			}
			for _, d := range dirs {
				if err := os.MkdirAll(d, 0o755); err != nil {
					fmt.Fprintf(os.Stderr, "%s: failed to create %s: %v\n", agent, d, err)
					continue
				}
			}
			fmt.Printf("%s: created mailbox/ tree\n", agent)

			// Migrate INBOX.md if it exists and has content
			inboxPath := filepath.Join(agentDir, "INBOX.md")
			inboxData, err := os.ReadFile(inboxPath)
			if err == nil && len(strings.TrimSpace(string(inboxData))) > 0 {
				ts := time.Now().Format("2006-01-02T15-04-05")
				msgName := fmt.Sprintf("%s-system-migrated-inbox.md", ts)
				msgContent := fmt.Sprintf("---\nfrom: system\nto: %s\ntimestamp: %s\ncategory: all\nsubject: Migrated from INBOX.md\ntype: notification\n---\n\n%s\n", agent, time.Now().Format(time.RFC3339), string(inboxData))

				msgPath := filepath.Join(mailboxDir, "inbox", "all", msgName)
				if err := os.WriteFile(msgPath, []byte(msgContent), 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "%s: failed to migrate INBOX.md: %v\n", agent, err)
				} else {
					fmt.Printf("%s: migrated INBOX.md → mailbox/inbox/all/%s\n", agent, msgName)
				}
			}
		}

		return nil
	},
}
