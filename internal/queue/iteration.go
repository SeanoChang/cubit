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
