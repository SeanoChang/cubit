package queue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "queue"), 0o755)
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "log.md"), []byte(""), 0o644)
	return dir
}

func TestCreateTask(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)

	task, err := q.Create("implement FTS5 insert", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID != 1 {
		t.Errorf("ID = %d, want 1", task.ID)
	}
	if task.Status != "pending" {
		t.Errorf("Status = %q, want pending", task.Status)
	}

	// File should exist
	pattern := filepath.Join(dir, "queue", "001-*.md")
	matches, _ := filepath.Glob(pattern)
	if len(matches) != 1 {
		t.Fatalf("expected 1 file matching %s, got %d", pattern, len(matches))
	}
}

func TestCreateTaskWithContext(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)

	task, err := q.Create("sweep arch", "baseline val_bpb: 0.997")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.Contains(task.Body, "baseline val_bpb: 0.997") {
		t.Errorf("Body missing context: %s", task.Body)
	}
}

func TestCreateAutoIncrements(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)

	q.Create("first", "")
	task2, _ := q.Create("second", "")
	if task2.ID != 2 {
		t.Errorf("second task ID = %d, want 2", task2.ID)
	}
}

func TestList(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)
	q.Create("first", "")
	q.Create("second", "")

	tasks, err := q.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("List len = %d, want 2", len(tasks))
	}
	if tasks[0].ID != 1 || tasks[1].ID != 2 {
		t.Errorf("List order wrong: %d, %d", tasks[0].ID, tasks[1].ID)
	}
}

func TestPop(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)
	q.Create("first", "")
	q.Create("second", "")

	task, err := q.Pop()
	if err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if task.ID != 1 {
		t.Errorf("Pop ID = %d, want 1", task.ID)
	}
	if task.Status != "doing" {
		t.Errorf("Pop Status = %q, want doing", task.Status)
	}

	// .doing should exist
	doingPath := filepath.Join(dir, "queue", ".doing")
	if _, err := os.Stat(doingPath); err != nil {
		t.Errorf(".doing file missing: %v", err)
	}

	// Original file should be gone
	matches, _ := filepath.Glob(filepath.Join(dir, "queue", "001-*.md"))
	if len(matches) != 0 {
		t.Errorf("original file still exists after pop")
	}
}

func TestPopRefusesWhenDoing(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)
	q.Create("first", "")
	q.Create("second", "")

	q.Pop()
	_, err := q.Pop()
	if err == nil {
		t.Error("expected error when .doing exists")
	}
}

func TestPopEmptyQueue(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)

	_, err := q.Pop()
	if err == nil {
		t.Error("expected error on empty queue")
	}
}

func TestComplete(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)
	q.Create("first", "")
	q.Pop()

	err := q.Complete("done with it")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// .doing should be gone
	doingPath := filepath.Join(dir, "queue", ".doing")
	if _, err := os.Stat(doingPath); !os.IsNotExist(err) {
		t.Error(".doing still exists after Complete")
	}

	// log.md should have entry
	logData, _ := os.ReadFile(filepath.Join(dir, "memory", "log.md"))
	if !strings.Contains(string(logData), "first") {
		t.Errorf("log.md missing task title: %s", logData)
	}
	if !strings.Contains(string(logData), "done with it") {
		t.Errorf("log.md missing summary: %s", logData)
	}
}

func TestRequeue(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)
	q.Create("first", "")
	q.Pop()

	err := q.Requeue()
	if err != nil {
		t.Fatalf("Requeue: %v", err)
	}

	// .doing should be gone
	doingPath := filepath.Join(dir, "queue", ".doing")
	if _, err := os.Stat(doingPath); !os.IsNotExist(err) {
		t.Error(".doing still exists after Requeue")
	}

	// Task should be back in queue as pending
	tasks, _ := q.List()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after requeue, got %d", len(tasks))
	}
	if tasks[0].Status != "pending" {
		t.Errorf("requeued task status = %q, want pending", tasks[0].Status)
	}
}

func TestLogObservation(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)

	err := q.Log("something interesting happened")
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	logData, _ := os.ReadFile(filepath.Join(dir, "memory", "log.md"))
	if !strings.Contains(string(logData), "something interesting happened") {
		t.Errorf("log.md missing observation: %s", logData)
	}
	if !strings.Contains(string(logData), "observation") {
		t.Errorf("log.md missing 'observation' label: %s", logData)
	}
}

func TestActiveTask(t *testing.T) {
	dir := setupTestDir(t)
	q := NewQueue(dir)

	// No active task
	task, err := q.Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if task != nil {
		t.Error("expected nil when no .doing")
	}

	// Pop creates active
	q.Create("first", "")
	q.Pop()
	task, err = q.Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if task == nil {
		t.Fatal("expected active task")
	}
	if task.ID != 1 {
		t.Errorf("Active ID = %d, want 1", task.ID)
	}
}
