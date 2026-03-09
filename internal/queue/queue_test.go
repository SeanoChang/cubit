package queue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	instance = nil // reset singleton between tests
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "queue"), 0o755)
	os.MkdirAll(filepath.Join(dir, "queue", ".doing"), 0o755)
	os.MkdirAll(filepath.Join(dir, "queue", "done"), 0o755)
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.MkdirAll(filepath.Join(dir, "scratch"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "log.md"), []byte(""), 0o644)
	return dir
}

func TestCreateTask(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)

	task, err := q.Create("implement FTS5 insert", CreateOptions{})
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
	q := GetQueue(dir)

	task, err := q.Create("sweep arch", CreateOptions{Context: "baseline val_bpb: 0.997"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.Contains(task.Body, "baseline val_bpb: 0.997") {
		t.Errorf("Body missing context: %s", task.Body)
	}
}

func TestCreateAutoIncrements(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)

	q.Create("first", CreateOptions{})
	task2, _ := q.Create("second", CreateOptions{})
	if task2.ID != 2 {
		t.Errorf("second task ID = %d, want 2", task2.ID)
	}
}

func TestCreateWithOptions(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)

	opts := CreateOptions{
		Mode:          "loop",
		DependsOn:     []int{1, 2},
		Program:       "sweep.md",
		Goal:          "val_bpb < 0.95",
		MaxIterations: 50,
		Branch:        "noah/sweep",
	}
	task, err := q.Create("arch sweep", opts)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.Mode != "loop" {
		t.Errorf("Mode = %q, want loop", task.Mode)
	}
	if len(task.DependsOn) != 2 || task.DependsOn[0] != 1 || task.DependsOn[1] != 2 {
		t.Errorf("DependsOn = %v, want [1 2]", task.DependsOn)
	}
	if task.Program != "sweep.md" {
		t.Errorf("Program = %q, want sweep.md", task.Program)
	}
	if task.Goal != "val_bpb < 0.95" {
		t.Errorf("Goal = %q", task.Goal)
	}
	if task.MaxIterations != 50 {
		t.Errorf("MaxIterations = %d, want 50", task.MaxIterations)
	}
	if task.Branch != "noah/sweep" {
		t.Errorf("Branch = %q, want noah/sweep", task.Branch)
	}

	// Round-trip: read back from disk
	tasks, err := q.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if tasks[0].Mode != "loop" {
		t.Errorf("persisted Mode = %q, want loop", tasks[0].Mode)
	}
}

func TestCreateDefaultsMode(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)

	task, err := q.Create("simple task", CreateOptions{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.Mode != "once" {
		t.Errorf("Mode = %q, want once (default)", task.Mode)
	}
}

func TestList(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})
	q.Create("second", CreateOptions{})

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
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})
	q.Create("second", CreateOptions{})

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

	matches, _ := filepath.Glob(filepath.Join(dir, "queue", ".doing", "001-*.md"))
	if len(matches) != 1 {
		t.Errorf("expected 1 file in .doing/, got %d", len(matches))
	}

	origMatches, _ := filepath.Glob(filepath.Join(dir, "queue", "001-*.md"))
	if len(origMatches) != 0 {
		t.Errorf("original file still exists after pop")
	}
}

func TestPopEmptyQueue(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)

	_, err := q.Pop()
	if err == nil {
		t.Error("expected error on empty queue")
	}
}

func TestComplete(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})
	q.Pop()

	err := q.Complete("done with it")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	doingMatches, _ := filepath.Glob(filepath.Join(dir, "queue", ".doing", "*.md"))
	if len(doingMatches) != 0 {
		t.Error(".doing/ still has files after Complete")
	}

	doneMatches, _ := filepath.Glob(filepath.Join(dir, "queue", "done", "001-*.md"))
	if len(doneMatches) != 1 {
		t.Fatalf("expected 1 done file, got %d", len(doneMatches))
	}

	logData, _ := os.ReadFile(filepath.Join(dir, "memory", "log.md"))
	if !strings.Contains(string(logData), "first") {
		t.Errorf("log.md missing task title")
	}
}

func TestListDone(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)

	q.Create("first", CreateOptions{})
	q.Create("second", CreateOptions{})
	q.Pop()
	q.Complete("first done")

	done, err := q.ListDone()
	if err != nil {
		t.Fatalf("ListDone: %v", err)
	}
	if len(done) != 1 {
		t.Fatalf("ListDone len = %d, want 1", len(done))
	}
	if done[0].ID != 1 {
		t.Errorf("done[0].ID = %d, want 1", done[0].ID)
	}
	if done[0].Status != "done" {
		t.Errorf("done[0].Status = %q, want done", done[0].Status)
	}
}

func TestRequeue(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})
	q.Pop()

	err := q.Requeue()
	if err != nil {
		t.Fatalf("Requeue: %v", err)
	}

	doingMatches, _ := filepath.Glob(filepath.Join(dir, "queue", ".doing", "*.md"))
	if len(doingMatches) != 0 {
		t.Error(".doing/ still has files after Requeue")
	}

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
	q := GetQueue(dir)

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
	q := GetQueue(dir)

	tasks, err := q.Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if len(tasks) != 0 {
		t.Error("expected empty when no .doing/ contents")
	}

	q.Create("first", CreateOptions{})
	q.Pop()
	tasks, err = q.Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatal("expected 1 active task")
	}
	if tasks[0].ID != 1 {
		t.Errorf("Active ID = %d, want 1", tasks[0].ID)
	}
}

func TestMultipleActiveTasks(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})
	q.Create("second", CreateOptions{})

	q.Pop()
	q.Pop()

	tasks, err := q.Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("want 2 active, got %d", len(tasks))
	}
}

func TestCompleteByID(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})
	q.Create("second", CreateOptions{})
	q.Pop()
	q.Pop()

	err := q.CompleteByID(1, "done with first")
	if err != nil {
		t.Fatalf("CompleteByID: %v", err)
	}

	active, _ := q.Active()
	if len(active) != 1 {
		t.Fatalf("want 1 active after complete, got %d", len(active))
	}
	if active[0].ID != 2 {
		t.Errorf("remaining active ID = %d, want 2", active[0].ID)
	}

	done, _ := q.ListDone()
	if len(done) != 1 || done[0].ID != 1 {
		t.Errorf("done list unexpected: %v", done)
	}
}

func TestRequeueByID(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})
	q.Create("second", CreateOptions{})
	q.Pop()
	q.Pop()

	err := q.RequeueByID(1)
	if err != nil {
		t.Fatalf("RequeueByID: %v", err)
	}

	active, _ := q.Active()
	if len(active) != 1 {
		t.Fatalf("want 1 active after requeue, got %d", len(active))
	}
	if active[0].ID != 2 {
		t.Errorf("remaining active ID = %d, want 2", active[0].ID)
	}

	pending, _ := q.List()
	if len(pending) != 1 || pending[0].ID != 1 {
		t.Errorf("requeued task not in pending list")
	}
}


func TestPopReady_SkipsBlocked(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("root", CreateOptions{})
	q.Create("depends on root", CreateOptions{DependsOn: []int{1}})

	task, err := q.PopReady()
	if err != nil {
		t.Fatalf("PopReady: %v", err)
	}
	if task.ID != 1 {
		t.Errorf("PopReady ID = %d, want 1 (the unblocked one)", task.ID)
	}
}

func TestPopReady_NoneReady(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("blocked", CreateOptions{DependsOn: []int{99}})

	_, err := q.PopReady()
	if err == nil {
		t.Error("expected error when no tasks are ready")
	}
}

func TestPopAllReady(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("a", CreateOptions{})
	q.Create("b", CreateOptions{})
	q.Create("blocked", CreateOptions{DependsOn: []int{1, 2}})

	tasks, err := q.PopAllReady()
	if err != nil {
		t.Fatalf("PopAllReady: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("want 2 popped, got %d", len(tasks))
	}

	pending, _ := q.List()
	if len(pending) != 1 || pending[0].ID != 3 {
		t.Errorf("expected task 3 still pending, got %v", pending)
	}
}

func TestPopAllReady_EmptyQueue(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)

	tasks, err := q.PopAllReady()
	if err != nil {
		t.Fatalf("PopAllReady: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("want 0, got %d", len(tasks))
	}
}

func TestValidateDependencies_NoCycle(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})
	if err := q.ValidateDependencies(2, []int{1}); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateDependencies_Cycle(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	// task 1 depends on task 2 (which doesn't exist yet)
	q.Create("first", CreateOptions{DependsOn: []int{2}})
	// now validate adding task 2 with dep on task 1 — would cycle
	if err := q.ValidateDependencies(2, []int{1}); err == nil {
		t.Error("expected cycle error, got nil")
	}
}
