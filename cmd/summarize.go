package cmd

import "strings"

// summarize returns the first line of output, truncated to 200 chars.
// Falls back to "completed" if output is empty.
func summarize(output string) string {
	line := strings.SplitN(strings.TrimSpace(output), "\n", 2)[0]
	line = strings.TrimSpace(line)
	if line == "" {
		return "completed"
	}
	if len(line) > 200 {
		line = line[:200]
	}
	return line
}
