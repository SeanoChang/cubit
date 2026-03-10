// Package brief assembles the session brief from agent files and provides utilities
package brief

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
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
		{"memory/MEMORY.md", ""},
		{"memory/brief.md", ""},
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

	// Active tasks from .doing/ directory
	doingDir := filepath.Join(agentDir, "queue", ".doing")
	doingFiles, _ := filepath.Glob(filepath.Join(doingDir, "*.md"))
	sort.Strings(doingFiles)
	if len(doingFiles) > 0 {
		var taskParts []string
		for _, f := range doingFiles {
			content := readFile(f)
			if content != "" {
				taskParts = append(taskParts, content)
			}
		}
		if len(taskParts) > 0 {
			active := "## Active Tasks\n" + strings.Join(taskParts, "\n\n")
			parts = append(parts, active)
		}
	}

	// Current plan
	if plan := readFile(filepath.Join(agentDir, "scratch", "plan.md")); plan != "" {
		parts = append(parts, "## Current Plan\n"+plan)
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// BuildWithUpstream builds the session brief and appends upstream output paths
// for fan-in nodes. upstreamIDs are task IDs whose outputs should be referenced.
func BuildWithUpstream(agentDir string, upstreamIDs []int) string {
	base := Build(agentDir)

	if len(upstreamIDs) == 0 {
		return base
	}

	var paths []string
	for _, id := range upstreamIDs {
		filename := fmt.Sprintf("%03d-output.md", id)
		relPath := filepath.Join("scratch", filename)
		absPath := filepath.Join(agentDir, relPath)
		if _, err := os.Stat(absPath); err == nil {
			paths = append(paths, "- "+relPath)
		}
	}

	if len(paths) == 0 {
		return base
	}

	upstream := "## Upstream Results\n" + strings.Join(paths, "\n")
	return base + "\n\n---\n\n" + upstream
}

// BuildLoopInjection builds the injection for a loop iteration.
// program is a path relative to agentDir (e.g. "sweep.md"). If empty, no program section.
// goal is the exit condition string. iteration is the current iteration number.
// maxIterations is the limit (0 = unlimited).
func BuildLoopInjection(agentDir, program, goal string, iteration, maxIterations int) string {
	base := Build(agentDir)

	var extra []string

	// Program file injection
	if program != "" {
		content := readFile(filepath.Join(agentDir, program))
		if content != "" {
			extra = append(extra, "## Program\n"+content)
		}
	}

	// Results context
	resultsPath := filepath.Join(agentDir, "memory", "results.tsv")
	if results := readFile(resultsPath); results != "" {
		extra = append(extra, "## Experiment Results\n```tsv\n"+results+"\n```")
	}

	// Iteration + goal info
	iterStr := fmt.Sprintf("Iteration %d", iteration)
	if maxIterations > 0 {
		iterStr = fmt.Sprintf("Iteration %d/%d", iteration, maxIterations)
	}

	goalBlock := iterStr
	if goal != "" {
		goalBlock += fmt.Sprintf("\nGoal: %s", goal)
		goalBlock += "\n\nWhen the goal is met, include the exact string GOAL_MET on its own line in your response."
	}
	extra = append(extra, "## Loop Status\n"+goalBlock)

	return base + "\n\n---\n\n" + strings.Join(extra, "\n\n---\n\n")
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
		{"memory/MEMORY.md", "Memory"},
		{"memory/brief.md", "Brief"},
	}

	var sections []Section
	for _, e := range entries {
		content := readFile(filepath.Join(agentDir, e.rel))
		sections = append(sections, Section{
			Label:   e.label,
			Content: content,
		})
	}

	// Active tasks from .doing/ directory
	doingDir := filepath.Join(agentDir, "queue", ".doing")
	doingFiles, _ := filepath.Glob(filepath.Join(doingDir, "*.md"))
	sort.Strings(doingFiles)
	var activeContent string
	for _, f := range doingFiles {
		content := readFile(f)
		if content != "" {
			if activeContent != "" {
				activeContent += "\n\n"
			}
			activeContent += content
		}
	}
	sections = append(sections, Section{Label: "Active Tasks", Content: activeContent})

	// Current plan
	sections = append(sections, Section{
		Label:   "Current Plan",
		Content: readFile(filepath.Join(agentDir, "scratch", "plan.md")),
	})

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
