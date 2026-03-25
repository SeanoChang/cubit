package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Info holds summary data about a project.
type Info struct {
	Name        string
	Path        string
	CommitCount int
	LastCommit  time.Time
	Branch      string
	HasEval     bool
}

// ProjectsDir returns the projects/ path for an agent workspace.
func ProjectsDir(agentDir string) string {
	return filepath.Join(agentDir, "projects")
}

// New creates a new project with git init.
func New(agentDir, name string) (string, error) {
	projDir := filepath.Join(ProjectsDir(agentDir), name)

	if _, err := os.Stat(projDir); err == nil {
		return "", fmt.Errorf("project %q already exists", name)
	}

	if err := os.MkdirAll(projDir, 0o755); err != nil {
		return "", fmt.Errorf("creating project directory: %w", err)
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = projDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(projDir)
		return "", fmt.Errorf("git init: %w", err)
	}

	// .gitignore
	gitignore := "*.tmp\n"
	if err := os.WriteFile(filepath.Join(projDir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return "", err
	}

	return projDir, nil
}

// List returns info about all projects in an agent workspace.
func List(agentDir string) ([]Info, error) {
	projsDir := ProjectsDir(agentDir)
	entries, err := os.ReadDir(projsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var projects []Info
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projPath := filepath.Join(projsDir, entry.Name())

		// Only include directories that are git repos
		if _, err := os.Stat(filepath.Join(projPath, ".git")); err != nil {
			continue
		}

		info := Info{
			Name: entry.Name(),
			Path: projPath,
		}

		// Commit count
		cmd := exec.Command("git", "rev-list", "--count", "HEAD")
		cmd.Dir = projPath
		if out, err := cmd.Output(); err == nil {
			fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &info.CommitCount)
		}

		// Last commit time
		cmd = exec.Command("git", "log", "-1", "--format=%ct")
		cmd.Dir = projPath
		if out, err := cmd.Output(); err == nil {
			var ts int64
			fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &ts)
			if ts > 0 {
				info.LastCommit = time.Unix(ts, 0)
			}
		}

		// Current branch
		cmd = exec.Command("git", "branch", "--show-current")
		cmd.Dir = projPath
		if out, err := cmd.Output(); err == nil {
			info.Branch = strings.TrimSpace(string(out))
		}

		// Check for EVAL.md
		if _, err := os.Stat(filepath.Join(projPath, "EVAL.md")); err == nil {
			info.HasEval = true
		}

		projects = append(projects, info)
	}

	// Sort by last activity (most recent first)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastCommit.After(projects[j].LastCommit)
	})

	return projects, nil
}

// Search searches across all project repos for matching content.
// Checks directory names, git log messages, and file content.
func Search(agentDir, query string) ([]SearchResult, error) {
	projsDir := ProjectsDir(agentDir)
	entries, err := os.ReadDir(projsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var results []SearchResult
	queryLower := strings.ToLower(query)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projPath := filepath.Join(projsDir, entry.Name())
		if _, err := os.Stat(filepath.Join(projPath, ".git")); err != nil {
			continue
		}

		var matches []string

		// Check directory name
		if strings.Contains(strings.ToLower(entry.Name()), queryLower) {
			matches = append(matches, "name match")
		}

		// Check git log messages
		cmd := exec.Command("git", "log", "--oneline", "--all", "--grep="+query, "-i")
		cmd.Dir = projPath
		if out, err := cmd.Output(); err == nil {
			lines := strings.TrimSpace(string(out))
			if lines != "" {
				count := len(strings.Split(lines, "\n"))
				matches = append(matches, fmt.Sprintf("%d commit(s)", count))
			}
		}

		// Check file content with git grep
		cmd = exec.Command("git", "grep", "-l", "-i", query)
		cmd.Dir = projPath
		if out, err := cmd.Output(); err == nil {
			lines := strings.TrimSpace(string(out))
			if lines != "" {
				count := len(strings.Split(lines, "\n"))
				matches = append(matches, fmt.Sprintf("%d file(s)", count))
			}
		}

		if len(matches) > 0 {
			results = append(results, SearchResult{
				Project: entry.Name(),
				Path:    projPath,
				Matches: matches,
			})
		}
	}

	return results, nil
}

// SearchResult holds search results for a single project.
type SearchResult struct {
	Project string
	Path    string
	Matches []string
}

// Archive archives a project by writing a summary to nark.
func Archive(agentDir, name string) error {
	projPath := filepath.Join(ProjectsDir(agentDir), name)
	if _, err := os.Stat(projPath); err != nil {
		return fmt.Errorf("project %q not found", name)
	}

	// Build summary from git log
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("# Project Archive: %s\n\n", name))

	cmd := exec.Command("git", "log", "--oneline", "-20")
	cmd.Dir = projPath
	if out, err := cmd.Output(); err == nil {
		summary.WriteString("## Recent Commits\n\n")
		summary.WriteString(string(out))
		summary.WriteString("\n")
	}

	// Send to nark
	title := fmt.Sprintf("project archive: %s", name)
	narkCmd := exec.Command("nark", "write", "--title", title)
	narkCmd.Stdin = strings.NewReader(summary.String())
	output, err := narkCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nark write failed: %v", err)
	}

	fmt.Printf("Archived project %q to nark: %s\n", name, strings.TrimSpace(string(output)))
	return nil
}

// Status returns detailed info about a single project.
func Status(agentDir, name string) (*Info, string, error) {
	projPath := filepath.Join(ProjectsDir(agentDir), name)
	if _, err := os.Stat(projPath); err != nil {
		return nil, "", fmt.Errorf("project %q not found", name)
	}

	info := &Info{
		Name: name,
		Path: projPath,
	}

	// Commit count
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = projPath
	if out, err := cmd.Output(); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &info.CommitCount)
	}

	// Last commit time
	cmd = exec.Command("git", "log", "-1", "--format=%ct")
	cmd.Dir = projPath
	if out, err := cmd.Output(); err == nil {
		var ts int64
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &ts)
		if ts > 0 {
			info.LastCommit = time.Unix(ts, 0)
		}
	}

	// Branch
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = projPath
	if out, err := cmd.Output(); err == nil {
		info.Branch = strings.TrimSpace(string(out))
	}

	// EVAL.md
	if _, err := os.Stat(filepath.Join(projPath, "EVAL.md")); err == nil {
		info.HasEval = true
	}

	// Git log for detail view
	var logOutput string
	cmd = exec.Command("git", "log", "--oneline", "-10")
	cmd.Dir = projPath
	if out, err := cmd.Output(); err == nil {
		logOutput = strings.TrimSpace(string(out))
	}

	return info, logOutput, nil
}

// FormatAge returns a human-readable age string.
func FormatAge(t time.Time) string {
	if t.IsZero() {
		return "no commits"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	default:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1w ago"
		}
		return fmt.Sprintf("%dw ago", weeks)
	}
}
