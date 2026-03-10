// Package claude wraps the claude CLI for prompt execution.
package claude

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RunnerOpts configures a Claude CLI invocation.
type RunnerOpts struct {
	Model          string
	PermissionMode string        // e.g. "dontAsk" for headless
	AllowedTools   []string      // tool whitelist
	Timeout        time.Duration // max execution time (0 = no timeout)
	WorkDir        string        // cwd for Claude process
}

// buildArgs constructs the CLI argument list from opts.
func buildArgs(opts RunnerOpts) []string {
	args := []string{"-p"}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.PermissionMode != "" {
		args = append(args, "--permission-mode", opts.PermissionMode)
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}
	return args
}

// Prompt sends a single-shot prompt to claude CLI and returns the response.
// Passes the prompt via stdin to avoid arg length limits.
func Prompt(ctx context.Context, prompt string, opts RunnerOpts) (string, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	args := buildArgs(opts)
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Stdin = strings.NewReader(prompt)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}
