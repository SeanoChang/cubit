package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send <draft-file>",
	Short: "Send a mailbox message to another agent",
	Long: `Reads a draft message with YAML frontmatter, validates it, and delivers
it to the target agent's mailbox/inbox/<category>/.

The draft is moved to your sent/ folder after delivery.

Required frontmatter fields: to, from, subject
Optional: category (important|priority|all, default: all), type (notification|request|handoff)

Example draft (mailbox/drafts/my-message.md):
  ---
  from: alice
  to: noah
  subject: Found a regression in auth module
  category: important
  type: notification
  ---

  Body of the message here.

Usage:
  cubit send mailbox/drafts/my-message.md
  cubit send --agent alice mailbox/drafts/my-message.md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !agentExplicit {
			return fmt.Errorf("agent not specified — use --agent <name> or run from inside an agent directory")
		}
		draftPath := args[0]
		if !filepath.IsAbs(draftPath) {
			draftPath = filepath.Join(cfg.AgentDir(), draftPath)
		}
		info, err := os.Stat(draftPath)
		if err != nil {
			return fmt.Errorf("reading draft: %w", err)
		}
		if info.IsDir() {
			return sendDirectoryMail(draftPath)
		}
		return sendFileMail(draftPath)
	},
}

// validateMailFields extracts and validates required frontmatter fields.
func validateMailFields(fm map[string]string) (to, from, subject, category string, err error) {
	to = fm["to"]
	from = fm["from"]
	subject = fm["subject"]

	if to == "" {
		return "", "", "", "", fmt.Errorf("missing required field: to")
	}
	if from == "" {
		return "", "", "", "", fmt.Errorf("missing required field: from")
	}
	if subject == "" {
		return "", "", "", "", fmt.Errorf("missing required field: subject")
	}

	if !isValidAgentName(to) {
		return "", "", "", "", fmt.Errorf("invalid agent name in 'to': %q", to)
	}
	if !isValidAgentName(from) {
		return "", "", "", "", fmt.Errorf("invalid agent name in 'from': %q", from)
	}

	category = fm["category"]
	if category == "" {
		category = "all"
	}
	validCategories := map[string]bool{"important": true, "priority": true, "all": true}
	if !validCategories[category] {
		return "", "", "", "", fmt.Errorf("invalid category %q — use: important, priority, all", category)
	}

	return to, from, subject, category, nil
}

// resolveTargetAgent resolves and validates the target agent's workspace directory.
func resolveTargetAgent(to string) (string, error) {
	agentsHome := filepath.Join(cfg.Root, "agents-home")
	targetDir := filepath.Join(agentsHome, to)
	if rel, err := filepath.Rel(agentsHome, targetDir); err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid target agent: %q", to)
	}
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return "", fmt.Errorf("unknown agent: %s (no workspace at %s)", to, targetDir)
	}
	return targetDir, nil
}

// sendFileMail sends a single .md draft file (existing behavior).
func sendFileMail(draftPath string) error {
	data, err := os.ReadFile(draftPath)
	if err != nil {
		return fmt.Errorf("reading draft: %w", err)
	}

	fm, body, err := parseFrontmatter(string(data))
	if err != nil {
		return fmt.Errorf("parsing frontmatter: %w", err)
	}

	to, from, subject, category, err := validateMailFields(fm)
	if err != nil {
		return err
	}

	targetDir, err := resolveTargetAgent(to)
	if err != nil {
		return err
	}

	now := time.Now()
	if fm["timestamp"] == "" {
		fm["timestamp"] = now.Format(time.RFC3339)
	}

	ts := now.Format("2006-01-02T15-04-05")
	slug := slugify(subject)
	canonicalName := fmt.Sprintf("%s-%s-%s.md", ts, from, slug)

	finalContent := buildMessage(fm, body)

	// Deliver to target inbox
	targetInbox := filepath.Join(targetDir, "mailbox", "inbox", category)
	if err := os.MkdirAll(targetInbox, 0o755); err != nil {
		return fmt.Errorf("creating target inbox: %w", err)
	}
	targetPath := filepath.Join(targetInbox, canonicalName)
	if err := os.WriteFile(targetPath, []byte(finalContent), 0o644); err != nil {
		return fmt.Errorf("delivering message: %w", err)
	}

	// Delivery succeeded — sent/draft cleanup is best-effort
	sentDir := filepath.Join(cfg.AgentDir(), "mailbox", "sent")
	if err := os.MkdirAll(sentDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create sent/: %v\n", err)
	} else {
		sentPath := filepath.Join(sentDir, canonicalName)
		if err := os.WriteFile(sentPath, []byte(finalContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write to sent/: %v\n", err)
		}
	}
	os.Remove(draftPath)

	fmt.Printf("Sent: %s → %s/mailbox/inbox/%s/%s\n", from, to, category, canonicalName)
	return nil
}

// sendDirectoryMail sends a directory-mail (directory containing mail.md plus attachments).
func sendDirectoryMail(draftPath string) error {
	mailPath := filepath.Join(draftPath, "mail.md")
	data, err := os.ReadFile(mailPath)
	if err != nil {
		return fmt.Errorf("reading directory-mail: %w (expected mail.md inside %s)", err, draftPath)
	}

	fm, body, err := parseFrontmatter(string(data))
	if err != nil {
		return fmt.Errorf("parsing frontmatter: %w", err)
	}

	to, from, subject, category, err := validateMailFields(fm)
	if err != nil {
		return err
	}

	targetDir, err := resolveTargetAgent(to)
	if err != nil {
		return err
	}

	now := time.Now()
	if fm["timestamp"] == "" {
		fm["timestamp"] = now.Format(time.RFC3339)
	}

	// Generate canonical directory name (no .md extension)
	ts := now.Format("2006-01-02T15-04-05")
	slug := slugify(subject)
	canonicalName := fmt.Sprintf("%s-%s-%s", ts, from, slug)

	// Update mail.md with final frontmatter
	finalContent := buildMessage(fm, body)
	if err := os.WriteFile(mailPath, []byte(finalContent), 0o644); err != nil {
		return fmt.Errorf("updating mail.md: %w", err)
	}

	// Deliver entire directory to target inbox via stage-then-rename.
	// This ensures fsnotify sees a complete directory (with mail.md already inside)
	// rather than a directory being populated incrementally.
	targetInbox := filepath.Join(targetDir, "mailbox", "inbox", category)
	if err := os.MkdirAll(targetInbox, 0o755); err != nil {
		return fmt.Errorf("creating target inbox: %w", err)
	}
	stagingPath := filepath.Join(targetInbox, ".staging-"+canonicalName)
	if err := copyDir(draftPath, stagingPath); err != nil {
		os.RemoveAll(stagingPath)
		return fmt.Errorf("staging directory-mail: %w", err)
	}
	targetPath := filepath.Join(targetInbox, canonicalName)
	if err := os.Rename(stagingPath, targetPath); err != nil {
		os.RemoveAll(stagingPath)
		return fmt.Errorf("delivering directory-mail: %w", err)
	}

	// Copy to sent/ (best-effort)
	sentDir := filepath.Join(cfg.AgentDir(), "mailbox", "sent")
	if err := os.MkdirAll(sentDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create sent/: %v\n", err)
	} else {
		sentPath := filepath.Join(sentDir, canonicalName)
		if err := copyDir(draftPath, sentPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not copy to sent/: %v\n", err)
		}
	}

	// Remove draft directory
	os.RemoveAll(draftPath)

	fmt.Printf("Sent: %s → %s/mailbox/inbox/%s/%s/\n", from, to, category, canonicalName)
	return nil
}

// parseFrontmatter extracts YAML frontmatter between --- delimiters.
// Returns field map and remaining body.
func parseFrontmatter(content string) (map[string]string, string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil, "", fmt.Errorf("no frontmatter found — file must start with ---")
	}

	// Find closing ---
	rest := content[3:]
	rest = strings.TrimLeft(rest, "\n\r")
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return nil, "", fmt.Errorf("unterminated frontmatter — missing closing ---")
	}

	fmBlock := rest[:end]
	body := strings.TrimLeft(rest[end+4:], "\n\r")

	fields := make(map[string]string)
	for _, line := range strings.Split(fmBlock, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		fields[key] = value
	}

	return fields, body, nil
}

// buildMessage reconstructs a message with frontmatter + body.
func buildMessage(fm map[string]string, body string) string {
	var sb strings.Builder
	sb.WriteString("---\n")

	// Write fields in a consistent order
	order := []string{"from", "to", "timestamp", "category", "subject", "type", "delegation_id", "attempt", "reply_to"}
	written := make(map[string]bool)
	for _, key := range order {
		if val, ok := fm[key]; ok && val != "" {
			fmt.Fprintf(&sb, "%s: %s\n", key, val)
			written[key] = true
		}
	}
	// Write any remaining fields
	for key, val := range fm {
		if !written[key] && val != "" {
			fmt.Fprintf(&sb, "%s: %s\n", key, val)
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a subject line to a URL-safe slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' {
			return r
		}
		return -1
	}, s)
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
		s = strings.TrimRight(s, "-")
	}
	return s
}


