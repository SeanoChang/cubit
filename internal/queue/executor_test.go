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
