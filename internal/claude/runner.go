// Package claude wraps the claude CLI for prompt execution.
package claude

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Prompt sends a single-shot prompt to claude CLI and returns the response.
// Passes the prompt via stdin to avoid arg length limits.
func Prompt(prompt string, model string) (string, error) {
	args := []string{"-p"}
	if model != "" {
		args = append(args, "--model", model)
	}
	cmd := exec.Command("claude", args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}
