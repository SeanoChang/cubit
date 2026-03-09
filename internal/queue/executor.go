package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TaskResult holds the outcome of executing a single task.
type TaskResult struct {
	TaskID  int
	Output  string
	Summary string
	Err     error
	Model   string
}

// Failed returns true if the task execution failed.
func (r TaskResult) Failed() bool {
	return r.Err != nil
}

// WriteTaskOutput writes a task's output to scratch/<NNN>-output.md.
func WriteTaskOutput(scratchDir string, taskID int, output string) error {
	filename := fmt.Sprintf("%03d-output.md", taskID)
	path := filepath.Join(scratchDir, filename)
	return os.WriteFile(path, []byte(output), 0o644)
}

// DeadlockError reports which tasks are stuck and why.
type DeadlockError struct {
	Stuck []*Task
}

func (e *DeadlockError) Error() string {
	var sb strings.Builder
	sb.WriteString("deadlock: no tasks can make progress\n")
	for _, t := range e.Stuck {
		fmt.Fprintf(&sb, "  %03d: %s (waiting on %v)\n", t.ID, t.Title, t.DependsOn)
	}
	return sb.String()
}
