package brief

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Section holds a labeled chunk of the assembled brief.
type Section struct {
	Label   string
	Content string
}

// Build reads agent files in order, skips missing, and joins them with
// \n\n---\n\n to produce the full session brief.
func Build(agentDir string) string {
	var parts []string

	// Fixed-order file list: path relative to agentDir, optional wrapper prefix.
	entries := []struct {
		rel    string
		prefix string
	}{
		{"identity/FLUCTLIGHT.md", ""},
		{"USER.md", ""},
		{"GOALS.md", ""},
		{"memory/brief.md", ""},
		{"queue/.doing", "## Active Task\n"},
		{"scratch/plan.md", "## Current Plan\n"},
	}

	for _, e := range entries {
		content := readFile(filepath.Join(agentDir, e.rel))
		if content == "" {
			continue
		}

		// Warn if memory brief exceeds token budget.
		if e.rel == "memory/brief.md" {
			if tok := EstimateTokens(content); tok > 30000 {
				log.Printf("warning: memory/brief.md is ~%d tokens (budget 30k)", tok)
			}
		}

		if e.prefix != "" {
			content = e.prefix + content
		}
		parts = append(parts, content)
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// EstimateTokens returns a rough token count: word count * 1.3.
func EstimateTokens(text string) int {
	return int(float64(len(strings.Fields(text))) * 1.3)
}

// Sections returns labeled sections for diagnostic display (e.g. cubit brief).
func Sections(agentDir string) []Section {
	entries := []struct {
		rel   string
		label string
	}{
		{"identity/FLUCTLIGHT.md", "FLUCTLIGHT"},
		{"USER.md", "USER"},
		{"GOALS.md", "GOALS"},
		{"memory/brief.md", "Brief"},
		{"queue/.doing", "Active Task"},
		{"scratch/plan.md", "Current Plan"},
	}

	var sections []Section
	for _, e := range entries {
		content := readFile(filepath.Join(agentDir, e.rel))
		sections = append(sections, Section{
			Label:   e.label,
			Content: content,
		})
	}
	return sections
}

// FormatTokens returns a human-readable token summary for a section.
func FormatTokens(content string) string {
	if content == "" {
		return "(none)"
	}
	return fmt.Sprintf("~%d tokens", EstimateTokens(content))
}

// readFile reads a file and returns its trimmed content, or "" on any error.
func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// tail returns the last n lines of text.
func tail(text string, n int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= n {
		return text
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
