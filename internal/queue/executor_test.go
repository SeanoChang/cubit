package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTaskOutput(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	err := WriteTaskOutput(scratchDir, 1, "hello world")
	if err != nil {
		t.Fatalf("WriteTaskOutput: %v", err)
	}

	path := filepath.Join(scratchDir, "001-output.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("output = %q, want %q", string(data), "hello world")
	}
}

func TestWriteTaskOutput_EmptyOutput(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	err := WriteTaskOutput(scratchDir, 5, "")
	if err != nil {
		t.Fatalf("WriteTaskOutput: %v", err)
	}

	path := filepath.Join(scratchDir, "005-output.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "" {
		t.Errorf("expected empty output, got %q", string(data))
	}
}

func TestTaskResult_Failed(t *testing.T) {
	ok := TaskResult{TaskID: 1, Output: "done"}
	if ok.Failed() {
		t.Error("expected not failed")
	}

	fail := TaskResult{TaskID: 2, Err: fmt.Errorf("timeout")}
	if !fail.Failed() {
		t.Error("expected failed")
	}
}

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

func TestDeadlockError(t *testing.T) {
	err := &DeadlockError{
		Stuck: []*Task{
			{ID: 3, Title: "blocked task", DependsOn: []int{1, 2}},
		},
	}
	msg := err.Error()
	if !strings.Contains(msg, "deadlock") {
		t.Error("missing 'deadlock' in error message")
	}
	if !strings.Contains(msg, "003") {
		t.Error("missing task ID in error message")
	}
}
