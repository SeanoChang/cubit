package claude

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Prompt sends a single-shot prompt to claude CLI and returns the response.
// Passes the prompt via stdin to avoid arg length limits.
func Prompt(prompt string) (string, error) {
	cmd := exec.Command("claude", "-p")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}
