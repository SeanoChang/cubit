package brief

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var narkIDPattern = regexp.MustCompile(`(?m)^nark:\s*([a-f0-9]+)`)

// ExtractNarkID parses a nark note ID from terminal node output.
// Returns empty string if not found.
func ExtractNarkID(output string) string {
	m := narkIDPattern.FindStringSubmatch(output)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// RunPostDrainLifecycle handles the close-out sequence after drain completes.
// terminalOutput is the output from the terminal summary node.
func RunPostDrainLifecycle(agentDir, terminalOutput string) error {
	narkID := ExtractNarkID(terminalOutput)
	date := time.Now().Format("2006-01-02")

	// Step 3: Log entry
	var logEntry string
	if narkID != "" {
		logEntry = fmt.Sprintf("\n## %s — goal cycle completed\nnark: %s\n", date, narkID)
	} else {
		logEntry = fmt.Sprintf("\n## %s — goal cycle completed\n(no nark ID found in terminal output)\n", date)
	}
	logPath := filepath.Join(agentDir, "memory", "log.md")
	if err := appendToFile(logPath, logEntry); err != nil {
		return fmt.Errorf("lifecycle: log entry: %w", err)
	}

	// Step 4: Clear GOALS.md
	goalsPath := filepath.Join(agentDir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(""), 0o644); err != nil {
		return fmt.Errorf("lifecycle: clear GOALS.md: %w", err)
	}

	// Step 5: Slim brief.md + update MEMORY.md
	var slimBrief string
	if narkID != "" {
		slimBrief = fmt.Sprintf("Previous cycle completed [%s]. nark: %s\n", date, narkID)
	} else {
		slimBrief = fmt.Sprintf("Previous cycle completed [%s].\n", date)
	}
	briefPath := filepath.Join(agentDir, "memory", "brief.md")
	if err := os.WriteFile(briefPath, []byte(slimBrief), 0o644); err != nil {
		return fmt.Errorf("lifecycle: slim brief.md: %w", err)
	}

	if narkID != "" {
		memoryPath := filepath.Join(agentDir, "memory", "MEMORY.md")
		pointer := fmt.Sprintf("\n- [%s] Archived to nark: %s\n", date, narkID)
		if err := appendToFile(memoryPath, pointer); err != nil {
			return fmt.Errorf("lifecycle: update MEMORY.md: %w", err)
		}
	}

	// Step 6: Clean scratch/
	scratchDir := filepath.Join(agentDir, "scratch")
	if err := cleanScratch(scratchDir); err != nil {
		return fmt.Errorf("lifecycle: clean scratch: %w", err)
	}

	return nil
}

func appendToFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func cleanScratch(dir string) error {
	entries, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return err
	}
	for _, f := range entries {
		os.Remove(f)
	}
	// Also clean iteration files
	txtEntries, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		return err
	}
	for _, f := range txtEntries {
		os.Remove(f)
	}
	return nil
}
