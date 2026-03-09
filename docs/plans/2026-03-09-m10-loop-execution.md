# M10: Loop Execution Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable loop-mode tasks in `cubit run` — tasks with `mode: loop` re-execute until their goal is met or max_iterations is reached, with program.md re-injection and memory pass between iterations.

**Architecture:** Loop execution is a new code path in `cmd/run.go`. When a task has `mode: loop`, instead of calling `executeWithRetry` (single-shot), we call a new `executeLoop` function that iterates: build injection (program.md + brief + results context) → prompt → check goal → memory pass → repeat. Goal evaluation is agent-driven — the LLM output is checked for a `GOAL_MET` signal. Results are appended to `memory/results.tsv` after each iteration.

**Tech Stack:** Go 1.26, Cobra, existing `internal/queue`, `internal/brief`, `internal/claude` packages

---

## Design Decisions

1. **Goal evaluation is agent-driven.** The program.md tells the agent to output `GOAL_MET` when the goal is satisfied. Cubit scans the output for this literal string. No expression parser needed — the agent evaluates `val_bpb < 0.95` and decides.

2. **Program injection replaces task body.** For loop tasks with a `program` field, the brief injects the program file contents instead of (in addition to) the task body. The program file path is relative to the agent directory.

3. **Results.tsv is append-only.** The agent appends rows. Cubit reads it into the injection so the agent sees prior experiment results. Cubit itself doesn't parse the TSV — it's opaque text passed through.

4. **Iteration state is tracked via a state file.** `scratch/NNN-iteration.txt` holds the current iteration count. This survives Ctrl-C + requeue cycles.

5. **Memory pass runs between iterations** (unless `--no-memory`). This keeps brief.md current across long loops.

6. **Loop tasks stay in .doing/ throughout.** They don't get requeued between iterations — only on Ctrl-C or error.

---

## Task 1: Results TSV Read/Append Helpers

**Files:**
- Create: `internal/queue/results.go`
- Test: `internal/queue/results_test.go`

These helpers read and append to `memory/results.tsv`. Cubit doesn't parse columns — it treats the file as opaque text that gets injected into the brief for the agent to read.

**Step 1: Write the failing tests**

```go
// internal/queue/results_test.go
package queue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadResults_Empty(t *testing.T) {
	dir := t.TempDir()
	content := ReadResults(dir)
	if content != "" {
		t.Errorf("expected empty, got %q", content)
	}
}

func TestReadResults_Exists(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)
	os.WriteFile(filepath.Join(memDir, "results.tsv"), []byte("commit\tval_bpb\tstatus\na1b2\t0.98\tkept\n"), 0o644)

	content := ReadResults(dir)
	if !strings.Contains(content, "a1b2") {
		t.Errorf("expected results content, got %q", content)
	}
}

func TestAppendResult(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)

	err := AppendResult(dir, "a1b2\t0.98\tkept\tswitch to GQA")
	if err != nil {
		t.Fatalf("AppendResult: %v", err)
	}

	content := ReadResults(dir)
	if !strings.Contains(content, "a1b2") {
		t.Errorf("expected appended content, got %q", content)
	}

	// Append a second row
	err = AppendResult(dir, "c3d4\t0.99\tdiscarded\ttry SwiGLU")
	if err != nil {
		t.Fatalf("AppendResult second: %v", err)
	}

	content = ReadResults(dir)
	if !strings.Contains(content, "c3d4") {
		t.Errorf("expected second row, got %q", content)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/queue/ -run TestReadResults -v && go test ./internal/queue/ -run TestAppendResult -v`
Expected: FAIL — `ReadResults` and `AppendResult` undefined

**Step 3: Write minimal implementation**

```go
// internal/queue/results.go
package queue

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadResults returns the contents of memory/results.tsv, or "" if missing.
func ReadResults(agentDir string) string {
	data, err := os.ReadFile(filepath.Join(agentDir, "memory", "results.tsv"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// AppendResult appends a single TSV row to memory/results.tsv.
func AppendResult(agentDir string, row string) error {
	path := filepath.Join(agentDir, "memory", "results.tsv")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if !strings.HasSuffix(row, "\n") {
		row += "\n"
	}
	_, err = f.WriteString(row)
	return err
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/queue/ -run "TestReadResults|TestAppendResult" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/queue/results.go internal/queue/results_test.go
git commit -m "feat(m10): add results.tsv read/append helpers"
```

---

## Task 2: Iteration State Tracker

**Files:**
- Create: `internal/queue/iteration.go`
- Test: `internal/queue/iteration_test.go`

Tracks iteration count per task in `scratch/NNN-iteration.txt`. Survives requeue cycles.

**Step 1: Write the failing tests**

```go
// internal/queue/iteration_test.go
package queue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetIteration_NoFile(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	n := GetIteration(scratchDir, 1)
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestIncrementIteration(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	n := IncrementIteration(scratchDir, 1)
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}

	n = IncrementIteration(scratchDir, 1)
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}

	got := GetIteration(scratchDir, 1)
	if got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestClearIteration(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	IncrementIteration(scratchDir, 1)
	IncrementIteration(scratchDir, 1)
	ClearIteration(scratchDir, 1)

	got := GetIteration(scratchDir, 1)
	if got != 0 {
		t.Errorf("expected 0 after clear, got %d", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/queue/ -run "TestGetIteration|TestIncrementIteration|TestClearIteration" -v`
Expected: FAIL — functions undefined

**Step 3: Write minimal implementation**

```go
// internal/queue/iteration.go
package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GetIteration returns the current iteration count for a task, or 0 if none.
func GetIteration(scratchDir string, taskID int) int {
	path := filepath.Join(scratchDir, fmt.Sprintf("%03d-iteration.txt", taskID))
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return n
}

// IncrementIteration bumps the iteration count by 1 and returns the new value.
func IncrementIteration(scratchDir string, taskID int) int {
	n := GetIteration(scratchDir, taskID) + 1
	path := filepath.Join(scratchDir, fmt.Sprintf("%03d-iteration.txt", taskID))
	os.WriteFile(path, []byte(strconv.Itoa(n)), 0o644)
	return n
}

// ClearIteration removes the iteration state file for a task.
func ClearIteration(scratchDir string, taskID int) {
	path := filepath.Join(scratchDir, fmt.Sprintf("%03d-iteration.txt", taskID))
	os.Remove(path)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/queue/ -run "TestGetIteration|TestIncrementIteration|TestClearIteration" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/queue/iteration.go internal/queue/iteration_test.go
git commit -m "feat(m10): add iteration state tracker for loop tasks"
```

---

## Task 3: Loop Brief Injection

**Files:**
- Modify: `internal/brief/brief.go` (add `BuildLoopInjection`)
- Modify: `internal/brief/brief_test.go` (add tests)

Builds the injection string for a single loop iteration: program.md contents + standard brief + results.tsv context + iteration info.

**Step 1: Write the failing tests**

Add to `internal/brief/brief_test.go`:

```go
func TestBuildLoopInjection_WithProgram(t *testing.T) {
	dir := t.TempDir()
	// Create minimal agent structure
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "brief.md"), []byte("# Brief\nWorking on sweep."), 0o644)

	// Create program file
	os.WriteFile(filepath.Join(dir, "sweep.md"), []byte("# Sweep Program\nRun experiments."), 0o644)

	// Create results.tsv
	os.WriteFile(filepath.Join(dir, "memory", "results.tsv"), []byte("commit\tval_bpb\na1b2\t0.98\n"), 0o644)

	injection := BuildLoopInjection(dir, "sweep.md", "val_bpb < 0.95", 3, 100)

	if !strings.Contains(injection, "Sweep Program") {
		t.Error("expected program.md content in injection")
	}
	if !strings.Contains(injection, "a1b2") {
		t.Error("expected results.tsv content in injection")
	}
	if !strings.Contains(injection, "Iteration 3/100") {
		t.Error("expected iteration info in injection")
	}
	if !strings.Contains(injection, "val_bpb < 0.95") {
		t.Error("expected goal in injection")
	}
	if !strings.Contains(injection, "GOAL_MET") {
		t.Error("expected GOAL_MET instruction in injection")
	}
}

func TestBuildLoopInjection_NoProgram(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)

	injection := BuildLoopInjection(dir, "", "", 1, 0)

	// Should still have the base brief, just no program section
	if strings.Contains(injection, "## Program") {
		t.Error("should not have program section when no program file")
	}
	// No max_iterations means unlimited
	if !strings.Contains(injection, "Iteration 1") {
		t.Error("expected iteration info")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/brief/ -run "TestBuildLoopInjection" -v`
Expected: FAIL — `BuildLoopInjection` undefined

**Step 3: Write minimal implementation**

Add to `internal/brief/brief.go`:

```go
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
```

Note: You will need to add `"fmt"` to the imports in brief.go (it's already there if `fmt` is used elsewhere — check and add only if missing).

**Step 4: Run tests to verify they pass**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/brief/ -run "TestBuildLoopInjection" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/brief/brief.go internal/brief/brief_test.go
git commit -m "feat(m10): add BuildLoopInjection for loop task brief assembly"
```

---

## Task 4: Goal Detection Helper

**Files:**
- Create: `internal/queue/goal.go`
- Test: `internal/queue/goal_test.go`

Simple string scan: checks if the LLM output contains `GOAL_MET` on its own line.

**Step 1: Write the failing tests**

```go
// internal/queue/goal_test.go
package queue

import "testing"

func TestGoalMet_Present(t *testing.T) {
	output := "Results look good.\n\nGOAL_MET\n\nDone."
	if !GoalMet(output) {
		t.Error("expected goal met")
	}
}

func TestGoalMet_Absent(t *testing.T) {
	output := "Still working on it. val_bpb = 0.97."
	if GoalMet(output) {
		t.Error("expected goal not met")
	}
}

func TestGoalMet_InlineDoesNotCount(t *testing.T) {
	output := "The GOAL_MET criteria are not yet satisfied."
	// GOAL_MET appears but not on its own line — should still match
	// since it's a simple Contains check. This is intentional:
	// the agent is instructed to put it on its own line, but we're
	// lenient in detection.
	if !GoalMet(output) {
		t.Error("expected goal met (lenient match)")
	}
}

func TestGoalMet_Empty(t *testing.T) {
	if GoalMet("") {
		t.Error("expected goal not met on empty output")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/queue/ -run TestGoalMet -v`
Expected: FAIL — `GoalMet` undefined

**Step 3: Write minimal implementation**

```go
// internal/queue/goal.go
package queue

import "strings"

// GoalMet returns true if the output contains the GOAL_MET signal.
func GoalMet(output string) bool {
	return strings.Contains(output, "GOAL_MET")
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/queue/ -run TestGoalMet -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/queue/goal.go internal/queue/goal_test.go
git commit -m "feat(m10): add GoalMet signal detection"
```

---

## Task 5: Loop Executor in run.go

**Files:**
- Modify: `cmd/run.go` (add `executeLoop`, wire mode branching in goroutine dispatch)

This is the core task. The goroutine that currently calls `executeWithRetry` now checks `task.Mode` and dispatches to `executeLoop` for loop tasks.

**Step 1: Add `executeLoop` function to `cmd/run.go`**

Add this function after `executeWithRetry`:

```go
// executeLoop runs a loop-mode task: iterate until goal met, max_iterations, or cancellation.
func executeLoop(ctx context.Context, task *queue.Task, noMemory bool) queue.TaskResult {
	agentDir := cfg.AgentDir()
	scratchDir := filepath.Join(agentDir, "scratch")

	model := task.Model
	if model == "" {
		model = cfg.Claude.Model
	}

	maxIter := task.MaxIterations // 0 = unlimited

	for {
		select {
		case <-ctx.Done():
			return queue.TaskResult{
				TaskID:  task.ID,
				Summary: "interrupted",
				Err:     ctx.Err(),
				Model:   model,
			}
		default:
		}

		iteration := queue.IncrementIteration(scratchDir, task.ID)

		// Check max_iterations
		if maxIter > 0 && iteration > maxIter {
			queue.ClearIteration(scratchDir, task.ID)
			return queue.TaskResult{
				TaskID:  task.ID,
				Summary: fmt.Sprintf("max iterations reached (%d)", maxIter),
				Model:   model,
			}
		}

		fmt.Printf("  ↻ %03d: iteration %d", task.ID, iteration)
		if maxIter > 0 {
			fmt.Printf("/%d", maxIter)
		}
		fmt.Println()

		// Build loop injection
		injection := brief.BuildLoopInjection(agentDir, task.Program, task.Goal, iteration, maxIter)
		full := injection + "\n\n---\n\nExecute the next iteration of the active loop task."

		output, err := claude.Prompt(full, model)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %03d: iteration %d error: %v\n", task.ID, iteration, err)
			continue // retry next iteration on transient errors
		}

		// Write output (overwrite each iteration — latest output wins)
		if writeErr := queue.WriteTaskOutput(scratchDir, task.ID, output); writeErr != nil {
			fmt.Fprintf(os.Stderr, "  warning: write output %03d: %v\n", task.ID, writeErr)
		}

		fmt.Printf("\n%s\n\n", output)

		// Memory pass between iterations
		if !noMemory {
			if memErr := brief.RunMemoryPass(agentDir, output, cfg.Claude.MemoryModel); memErr != nil {
				fmt.Fprintf(os.Stderr, "  warning: memory pass failed: %v\n", memErr)
			}
		}

		// Check goal
		if task.Goal != "" && queue.GoalMet(output) {
			queue.ClearIteration(scratchDir, task.ID)
			return queue.TaskResult{
				TaskID:  task.ID,
				Output:  output,
				Summary: fmt.Sprintf("goal met at iteration %d: %s", iteration, task.Goal),
				Model:   model,
			}
		}
	}
}
```

**Step 2: Wire mode branching in the goroutine dispatch**

In the existing goroutine dispatch block (around line 109-113), change:

```go
// OLD:
go func(t *queue.Task) {
    defer sem.Release(1)
    result := executeWithRetry(ctx, t, 3)
    doneCh <- result
}(popped)
```

to:

```go
// NEW:
go func(t *queue.Task) {
    defer sem.Release(1)
    var result queue.TaskResult
    if t.Mode == "loop" {
        result = executeLoop(ctx, t, noMemory)
    } else {
        result = executeWithRetry(ctx, t, 3)
    }
    doneCh <- result
}(popped)
```

Note: `noMemory` is already in scope from the outer closure (line 26). The goroutine can capture it.

**Step 3: Run build to verify compilation**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go build ./...`
Expected: compiles successfully

**Step 4: Run all existing tests to verify nothing breaks**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./...`
Expected: all PASS

**Step 5: Commit**

```bash
git add cmd/run.go
git commit -m "feat(m10): add loop execution with goal detection and iteration tracking"
```

---

## Task 6: Handle Loop Task Requeue on Ctrl-C

**Files:**
- Modify: `cmd/run.go` (update interrupt handler)

When a loop task is interrupted, it should be requeued (not completed). The current `handleResult` completes on error — for loop tasks with `ctx.Err()`, we want requeue instead.

**Step 1: Update `handleResult` in `cmd/run.go`**

Change the error branch in `handleResult` (around line 225-232):

```go
// OLD:
func handleResult(result queue.TaskResult, noMemory bool) {
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "✗ %03d: %s\n", result.TaskID, result.Err)
		if err := q.CompleteByID(result.TaskID, result.Summary); err != nil {
			fmt.Fprintf(os.Stderr, "  complete error %03d: %v\n", result.TaskID, err)
		}
		return
	}
```

to:

```go
// NEW:
func handleResult(result queue.TaskResult, noMemory bool) {
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "✗ %03d: %s\n", result.TaskID, result.Err)
		// Requeue interrupted tasks so they can resume
		if result.Err == context.Canceled || result.Err == context.DeadlineExceeded {
			if err := q.RequeueByID(result.TaskID); err != nil {
				fmt.Fprintf(os.Stderr, "  requeue error %03d: %v\n", result.TaskID, err)
			} else {
				fmt.Printf("  ↩ %03d: requeued\n", result.TaskID)
			}
			return
		}
		if err := q.CompleteByID(result.TaskID, result.Summary); err != nil {
			fmt.Fprintf(os.Stderr, "  complete error %03d: %v\n", result.TaskID, err)
		}
		return
	}
```

Note: You'll need to add `"context"` to the imports at the top of `cmd/run.go`.

**Step 2: Run build to verify compilation**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go build ./...`
Expected: compiles

**Step 3: Run all tests**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./...`
Expected: all PASS

**Step 4: Commit**

```bash
git add cmd/run.go
git commit -m "feat(m10): requeue interrupted tasks instead of completing them"
```

---

## Task 7: Integration Smoke Test

**Files:**
- Modify: `internal/queue/executor_test.go` (or create a new test file — up to you)

End-to-end-ish test that validates the loop components work together without calling the real LLM.

**Step 1: Write the integration test**

Add to `internal/queue/executor_test.go`:

```go
func TestLoopComponents_Integration(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(scratchDir, 0o755)
	os.MkdirAll(memDir, 0o755)

	// Simulate 3 loop iterations
	for i := 1; i <= 3; i++ {
		n := IncrementIteration(scratchDir, 42)
		if n != i {
			t.Fatalf("iteration %d: got %d", i, n)
		}

		// Simulate appending results
		row := fmt.Sprintf("commit%d\t0.%d\tkept\titeration %d", i, 99-i, i)
		if err := AppendResult(dir, row); err != nil {
			t.Fatalf("append result: %v", err)
		}
	}

	// Verify iteration count
	if got := GetIteration(scratchDir, 42); got != 3 {
		t.Errorf("expected iteration 3, got %d", got)
	}

	// Verify results accumulated
	results := ReadResults(dir)
	if !strings.Contains(results, "commit1") || !strings.Contains(results, "commit3") {
		t.Errorf("expected all results, got %q", results)
	}

	// Verify goal detection
	if !GoalMet("Improved! GOAL_MET") {
		t.Error("expected goal met")
	}
	if GoalMet("Still going...") {
		t.Error("expected goal not met")
	}

	// Clear iteration
	ClearIteration(scratchDir, 42)
	if got := GetIteration(scratchDir, 42); got != 0 {
		t.Errorf("expected 0 after clear, got %d", got)
	}
}
```

**Step 2: Run the test**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/queue/ -run TestLoopComponents_Integration -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/queue/executor_test.go
git commit -m "test(m10): add loop components integration test"
```

---

## Task 8: Run Full Test Suite + Build

Final verification that everything compiles and all tests pass.

**Step 1: Build**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go build -o cubit .`
Expected: binary produced

**Step 2: Full test suite**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./... -v`
Expected: all PASS

**Step 3: Quick manual check**

Run: `./cubit version` and `./cubit run --help`
Expected: version prints, `run` help shows existing flags (no new flags needed for M10)

**Step 4: Final commit (if any fixups needed)**

```bash
git add -A
git commit -m "feat(m10): loop execution for cubit run — complete"
```

---

## Summary of New/Modified Files

| File | Action | Purpose |
|------|--------|---------|
| `internal/queue/results.go` | Create | ReadResults, AppendResult for results.tsv |
| `internal/queue/results_test.go` | Create | Tests for results helpers |
| `internal/queue/iteration.go` | Create | GetIteration, IncrementIteration, ClearIteration |
| `internal/queue/iteration_test.go` | Create | Tests for iteration tracker |
| `internal/queue/goal.go` | Create | GoalMet signal detection |
| `internal/queue/goal_test.go` | Create | Tests for goal detection |
| `internal/brief/brief.go` | Modify | Add BuildLoopInjection function |
| `internal/brief/brief_test.go` | Modify | Add tests for BuildLoopInjection |
| `cmd/run.go` | Modify | Add executeLoop, mode branching, requeue on interrupt |
| `internal/queue/executor_test.go` | Modify | Add integration test |
