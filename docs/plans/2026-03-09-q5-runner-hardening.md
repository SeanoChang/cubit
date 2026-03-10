# Q5: Runner Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor `claude.Prompt()` to accept `context.Context` + `RunnerOpts` for headless operation (Discord bot, CI, cron).

**Architecture:** Replace `Prompt(prompt, model)` with `Prompt(ctx, prompt, opts RunnerOpts)`. Extract `buildArgs(opts)` as a testable helper. Add `PermissionMode`, `AllowedTools`, `WorkDir` to config. Update all 6 call sites. `ClaudeConfig` gets a `RunnerOpts()` convenience method.

**Tech Stack:** Go stdlib (`os/exec`, `context`, `strings`)

---

### Task 1: Add RunnerOpts and refactor Prompt()

**Files:**
- Modify: `internal/claude/runner.go`
- Create: `internal/claude/runner_test.go`

**Step 1: Write the test for buildArgs**

Create `internal/claude/runner_test.go`:

```go
package claude

import (
	"testing"
)

func TestBuildArgs(t *testing.T) {
	tests := []struct {
		name string
		opts RunnerOpts
		want []string
	}{
		{
			name: "minimal",
			opts: RunnerOpts{},
			want: []string{"-p"},
		},
		{
			name: "model only",
			opts: RunnerOpts{Model: "claude-sonnet-4-6"},
			want: []string{"-p", "--model", "claude-sonnet-4-6"},
		},
		{
			name: "all fields",
			opts: RunnerOpts{
				Model:          "claude-opus-4-6",
				PermissionMode: "dontAsk",
				AllowedTools:   []string{"Bash", "Read", "Write"},
			},
			want: []string{"-p", "--model", "claude-opus-4-6", "--permission-mode", "dontAsk", "--allowedTools", "Bash,Read,Write"},
		},
		{
			name: "permission mode only",
			opts: RunnerOpts{PermissionMode: "dontAsk"},
			want: []string{"-p", "--permission-mode", "dontAsk"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildArgs(tt.opts)
			if len(got) != len(tt.want) {
				t.Fatalf("buildArgs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/claude/ -v -run TestBuildArgs
```
Expected: FAIL — `buildArgs` and `RunnerOpts` don't exist yet.

**Step 3: Rewrite runner.go**

Replace `internal/claude/runner.go` with:

```go
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
	// Apply timeout if set
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
```

**Step 4: Run tests**

```bash
go test ./internal/claude/ -v -run TestBuildArgs
```
Expected: PASS.

---

### Task 2: Expand ClaudeConfig with new fields + RunnerOpts() helper

**Files:**
- Modify: `internal/config/config.go:18-24` (ClaudeConfig struct)
- Modify: `internal/config/config.go:40-44` (defaults)
- Modify: `internal/config/config.go:77-83` (Default func)

**Step 1: Add fields to ClaudeConfig**

In `internal/config/config.go`, update the `ClaudeConfig` struct (lines 18-24):

```go
type ClaudeConfig struct {
	Model           string        `yaml:"model"            mapstructure:"model"`
	Timeout         time.Duration `yaml:"timeout"          mapstructure:"timeout"`
	MemoryModel     string        `yaml:"memory_model"     mapstructure:"memory_model"`
	RefreshJournals int           `yaml:"refresh_journals" mapstructure:"refresh_journals"`
	MaxParallel     int           `yaml:"max_parallel"     mapstructure:"max_parallel"`
	PermissionMode  string        `yaml:"permission_mode"  mapstructure:"permission_mode"`
	AllowedTools    []string      `yaml:"allowed_tools"    mapstructure:"allowed_tools"`
	WorkDir         string        `yaml:"work_dir"         mapstructure:"work_dir"`
}
```

**Step 2: Add RunnerOpts() method**

Add after the `AgentDir()` method, at the end of the file:

```go
// RunnerOpts returns a claude.RunnerOpts populated from config.
// Callers can override individual fields (e.g. Model) after calling this.
func (c *ClaudeConfig) RunnerOpts() claude.RunnerOpts {
	return claude.RunnerOpts{
		Model:          c.Model,
		PermissionMode: c.PermissionMode,
		AllowedTools:   c.AllowedTools,
		Timeout:        c.Timeout,
		WorkDir:        c.WorkDir,
	}
}
```

Add the import for `claude`:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SeanoChang/cubit/internal/claude"
	"github.com/spf13/viper"
)
```

**Step 3: Verify it compiles**

```bash
go build ./internal/config/
```
Expected: success (no errors). Note: the Viper defaults and `Default()` func don't need new entries — empty string / nil slice / zero duration are the correct defaults for the new fields.

---

### Task 3: Update call sites — cmd/prompt.go

**Files:**
- Modify: `cmd/prompt.go`

**Step 1: Update prompt.go**

Replace `cmd/prompt.go` entirely:

```go
package cmd

import (
	"context"
	"fmt"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/SeanoChang/cubit/internal/claude"
	"github.com/spf13/cobra"
)

var promptCmd = &cobra.Command{
	Use:   `prompt "<message>"`,
	Short: "Single-shot prompt with brief injection and memory pass",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		injection := brief.Build(cfg.AgentDir())
		full := injection + "\n\n---\n\n" + args[0]

		opts := cfg.Claude.RunnerOpts()
		result, err := claude.Prompt(context.Background(), full, opts)
		if err != nil {
			return err
		}

		fmt.Printf("\n%s\n", result)

		noMemory, _ := cmd.Flags().GetBool("no-memory")
		if !noMemory {
			if err := brief.RunMemoryPass(context.Background(), cfg.AgentDir(), result, cfg.Claude.MemoryModel); err != nil {
				fmt.Printf("warning: memory pass failed: %v\n", err)
			}
		}

		return nil
	},
}
```

**Step 2: Verify it compiles**

```bash
go build ./cmd/...
```
Expected: will fail because `brief.RunMemoryPass` doesn't accept `context.Context` yet. That's Task 5. Proceed to Task 4.

---

### Task 4: Update call sites — cmd/run.go

**Files:**
- Modify: `cmd/run.go:200` (executeWithRetry)
- Modify: `cmd/run.go:276` (executeLoop)
- Modify: `cmd/run.go:291` (memory pass in executeLoop)
- Modify: `cmd/run.go:336` (memory pass in handleResult)

**Step 1: Update executeWithRetry**

In `cmd/run.go`, replace the `claude.Prompt` call in `executeWithRetry` (around line 200):

Old:
```go
		output, err := claude.Prompt(full, model)
```

New:
```go
		opts := cfg.Claude.RunnerOpts()
		opts.Model = model
		output, err := claude.Prompt(ctx, full, opts)
```

**Step 2: Update executeLoop**

Replace the `claude.Prompt` call in `executeLoop` (around line 276):

Old:
```go
		output, err := claude.Prompt(full, model)
```

New:
```go
		opts := cfg.Claude.RunnerOpts()
		opts.Model = model
		output, err := claude.Prompt(ctx, full, opts)
```

**Step 3: Update memory pass calls in executeLoop and handleResult**

In `executeLoop` (around line 291), change:
```go
			if memErr := brief.RunMemoryPass(agentDir, output, cfg.Claude.MemoryModel); memErr != nil {
```
to:
```go
			if memErr := brief.RunMemoryPass(ctx, agentDir, output, cfg.Claude.MemoryModel); memErr != nil {
```

In `handleResult` (around line 336), change:
```go
		if err := brief.RunMemoryPass(cfg.AgentDir(), result.Output, cfg.Claude.MemoryModel); err != nil {
```
to:
```go
		if err := brief.RunMemoryPass(context.Background(), cfg.AgentDir(), result.Output, cfg.Claude.MemoryModel); err != nil {
```

Note: `handleResult` doesn't have a ctx parameter. Use `context.Background()` here — it's called after the task is already done, so cancellation isn't critical.

---

### Task 5: Update call sites — internal/brief/memory.go

**Files:**
- Modify: `internal/brief/memory.go:37` (RunMemoryPass signature)
- Modify: `internal/brief/memory.go:40` (Prompt call)
- Modify: `internal/brief/memory.go:83` (RunRefresh signature)
- Modify: `internal/brief/memory.go:86` (Prompt call)

**Step 1: Update RunMemoryPass**

Change signature (line 37):
```go
func RunMemoryPass(ctx context.Context, agentDir, rawOutput, model string) error {
```

Change Prompt call (line 40):
```go
	result, err := claude.Prompt(ctx, prompt, claude.RunnerOpts{Model: model})
```

Add `"context"` to imports.

**Step 2: Update RunRefresh**

Change signature (line 83):
```go
func RunRefresh(ctx context.Context, agentDir, model string, numJournals int) error {
```

Change Prompt call (line 86):
```go
	result, err := claude.Prompt(ctx, prompt, claude.RunnerOpts{Model: model})
```

---

### Task 6: Update call sites — internal/scaffold/setup.go + cmd/refresh.go

**Files:**
- Modify: `internal/scaffold/setup.go:85` (first Prompt call)
- Modify: `internal/scaffold/setup.go:160` (second Prompt call)
- Modify: `cmd/refresh.go` (RunRefresh call)

**Step 1: Update setup.go Prompt calls**

Line 85 — change:
```go
		response, err := claude.Prompt(prompt, "")
```
to:
```go
		response, err := claude.Prompt(ctx, prompt, claude.RunnerOpts{})
```

Line 160 — change:
```go
		content, err := claude.Prompt(prompt, "")
```
to:
```go
		content, err := claude.Prompt(ctx, prompt, claude.RunnerOpts{})
```

Add `"context"` to imports if not already present (it already is — `context` is imported on line 5).

**Step 2: Update cmd/refresh.go**

Find the `RunRefresh` call and add `context.Background()` as the first argument. (Read the file to confirm exact line.)

**Step 3: Build and test**

```bash
go build -o cubit .
go test ./internal/claude/ -v
go test ./internal/updater/ -v
go test ./internal/queue/ -v
```
Expected: all pass, binary builds.

**Step 4: Verify cubit still works**

```bash
./cubit --version
./cubit status
```
Expected: both work correctly.
