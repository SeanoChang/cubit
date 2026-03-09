# M9 — Concurrent DAG Executor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the linear `cubit run` executor with a concurrent DAG executor that fans out independent tasks and fans in at dependency boundaries.

**Architecture:** Event-driven scheduler in main goroutine dispatches ready tasks to worker goroutines gated by a semaphore. Workers are pure functions (build brief → call claude → return result). Main goroutine owns all graph state — no mutex needed.

**Tech Stack:** Go stdlib (`sync`, `context`, `os/signal`), Cobra, existing `internal/queue` + `internal/brief` + `internal/claude` packages.

**Design doc:** `docs/plans/2026-03-09-m9-concurrent-dag-executor-design.md`

---

### Task 1: Add `MaxParallel` to config

**Files:**
- Modify: `internal/config/config.go:18-23` (ClaudeConfig struct)
- Modify: `internal/config/config.go:33-43` (Load defaults)
- Modify: `internal/config/config.go:71-82` (Default func)

**Step 1: Add the field and defaults**

In `ClaudeConfig`, add:
```go
MaxParallel int `yaml:"max_parallel" mapstructure:"max_parallel"`
```

In `Load()`, add default:
```go
v.SetDefault("claude.max_parallel", 0)
```

In `Default()`, add:
```go
MaxParallel: 0,
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: clean build

**Step 3: Commit**

```
feat(config): add MaxParallel field to ClaudeConfig
```

---

### Task 2: Change `BuildGraph` to accept multiple active tasks

Currently `BuildGraph(pending []*Task, active *Task, done []*Task)` takes a single active task. With `.doing/` directory, there can be multiple. Change the signature first since many things depend on it.

**Files:**
- Modify: `internal/queue/graph.go:29-61` (BuildGraph signature + body)
- Modify: `internal/queue/graph_test.go` (all BuildGraph test calls)
- Modify: `internal/queue/queue.go:261-298` (ValidateDependencies)
- Modify: `cmd/graph.go:27-36` (graphCmd)

**Step 1: Write tests for multi-active BuildGraph**

Add to `internal/queue/graph_test.go`:
```go
func TestBuildGraph_MultipleActive(t *testing.T) {
	done := []*Task{
		{ID: 1, Title: "setup", Mode: "once", Status: "done", DependsOn: []int{}},
	}
	active := []*Task{
		{ID: 2, Title: "task a", Mode: "once", Status: "doing", DependsOn: []int{1}},
		{ID: 3, Title: "task b", Mode: "once", Status: "doing", DependsOn: []int{1}},
	}
	pending := []*Task{
		{ID: 4, Title: "merge", Mode: "once", Status: "pending", DependsOn: []int{2, 3}},
	}

	nodes := BuildGraph(pending, active, done)

	want := map[int]NodeStatus{
		1: StatusDone,
		2: StatusActive,
		3: StatusActive,
		4: StatusWaiting, // deps 2,3 are active, not done
	}
	if len(nodes) != 4 {
		t.Fatalf("want 4 nodes, got %d", len(nodes))
	}
	for _, n := range nodes {
		if n.Status != want[n.Task.ID] {
			t.Errorf("node %d: status = %q, want %q", n.Task.ID, n.Status, want[n.Task.ID])
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/queue/ -run TestBuildGraph_MultipleActive -v`
Expected: FAIL (compilation error — wrong arg type)

**Step 3: Change BuildGraph signature**

In `graph.go`, change:
```go
func BuildGraph(pending []*Task, active *Task, done []*Task) []*GraphNode {
```
to:
```go
func BuildGraph(pending []*Task, active []*Task, done []*Task) []*GraphNode {
```

Replace the active block:
```go
	if active != nil {
		nodes = append(nodes, &GraphNode{Task: active, Status: StatusActive})
	}
```
with:
```go
	for _, t := range active {
		nodes = append(nodes, &GraphNode{Task: t, Status: StatusActive})
	}
```

**Step 4: Fix all existing test calls**

Every existing test that passes `active *Task` or `nil` needs updating:

- `TestBuildGraph_AllStatuses`: change `active` arg from `&Task{...}` to `[]*Task{{...}}`
- `TestBuildGraph_NoActive`: change `nil` to `nil` (nil slice is fine for `[]*Task`)
- `TestBuildGraph_OrderByID`: change `&Task{...}` to `[]*Task{{...}}`
- `TestBuildGraph_EmptyGraph`: no change (nil works)
- `TestBuildGraph_WaitingOnActive`: change `&Task{...}` to `[]*Task{{...}}`

**Step 5: Fix ValidateDependencies in queue.go**

Change:
```go
	active, err := q.Active()
	if err != nil {
		return err
	}
```
and:
```go
	if active != nil {
		knownIDs[active.ID] = true
	}
```
to:
```go
	activeTasks, err := q.Active()
	if err != nil {
		return err
	}
```
and:
```go
	for _, t := range activeTasks {
		knownIDs[t.ID] = true
	}
```
and the BuildGraph call from:
```go
	nodes := BuildGraph(pending, active, done)
```
to:
```go
	nodes := BuildGraph(pending, activeTasks, done)
```

**Note:** `q.Active()` still returns `*Task` at this point. Wrap it:
```go
	active, err := q.Active()
	if err != nil {
		return err
	}
	var activeTasks []*Task
	if active != nil {
		activeTasks = []*Task{active}
	}
```

**Step 6: Fix cmd/graph.go**

Same wrapping pattern:
```go
	active, err := q.Active()
	if err != nil {
		return err
	}
	var activeTasks []*Task
	if active != nil {
		activeTasks = []*Task{active}
	}
	nodes := queue.BuildGraph(pending, activeTasks, done)
```

**Step 7: Run all tests**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 8: Commit**

```
refactor(graph): BuildGraph accepts multiple active tasks
```

---

### Task 3: Add `ReadyNodes()` and `GraphComplete()` to graph.go

**Files:**
- Modify: `internal/queue/graph.go` (add two functions)
- Modify: `internal/queue/graph_test.go` (add tests)

**Step 1: Write failing tests**

Add to `graph_test.go`:
```go
func TestReadyNodes_BasicDAG(t *testing.T) {
	done := []*Task{
		{ID: 1, Title: "setup", Mode: "once", Status: "done", DependsOn: []int{}},
	}
	active := []*Task{
		{ID: 2, Title: "running", Mode: "once", Status: "doing", DependsOn: []int{1}},
	}
	pending := []*Task{
		{ID: 3, Title: "also ready", Mode: "once", Status: "pending", DependsOn: []int{1}},
		{ID: 4, Title: "waiting", Mode: "once", Status: "pending", DependsOn: []int{2, 3}},
	}

	ready := ReadyNodes(pending, active, done)

	if len(ready) != 1 {
		t.Fatalf("want 1 ready node, got %d", len(ready))
	}
	if ready[0].ID != 3 {
		t.Errorf("ready[0].ID = %d, want 3", ready[0].ID)
	}
}

func TestReadyNodes_MultipleFanOut(t *testing.T) {
	done := []*Task{
		{ID: 1, Title: "root", Mode: "once", Status: "done", DependsOn: []int{}},
	}
	pending := []*Task{
		{ID: 2, Title: "branch a", Mode: "once", Status: "pending", DependsOn: []int{1}},
		{ID: 3, Title: "branch b", Mode: "once", Status: "pending", DependsOn: []int{1}},
		{ID: 4, Title: "branch c", Mode: "once", Status: "pending", DependsOn: []int{1}},
	}

	ready := ReadyNodes(pending, nil, done)

	if len(ready) != 3 {
		t.Fatalf("want 3 ready nodes, got %d", len(ready))
	}
}

func TestReadyNodes_NoDeps(t *testing.T) {
	pending := []*Task{
		{ID: 1, Title: "a", Mode: "once", Status: "pending", DependsOn: []int{}},
		{ID: 2, Title: "b", Mode: "once", Status: "pending", DependsOn: []int{}},
	}

	ready := ReadyNodes(pending, nil, nil)
	if len(ready) != 2 {
		t.Fatalf("want 2 ready, got %d", len(ready))
	}
}

func TestReadyNodes_Empty(t *testing.T) {
	ready := ReadyNodes(nil, nil, nil)
	if len(ready) != 0 {
		t.Fatalf("want 0 ready, got %d", len(ready))
	}
}

func TestGraphComplete_AllDone(t *testing.T) {
	done := []*Task{
		{ID: 1, Status: "done"},
		{ID: 2, Status: "done"},
	}
	if !GraphComplete(nil, nil, done) {
		t.Error("expected complete when all done and nothing pending/active")
	}
}

func TestGraphComplete_StillPending(t *testing.T) {
	done := []*Task{{ID: 1, Status: "done"}}
	pending := []*Task{{ID: 2, Status: "pending", DependsOn: []int{1}}}
	if GraphComplete(pending, nil, done) {
		t.Error("expected not complete with pending tasks")
	}
}

func TestGraphComplete_StillActive(t *testing.T) {
	active := []*Task{{ID: 1, Status: "doing"}}
	if GraphComplete(nil, active, nil) {
		t.Error("expected not complete with active tasks")
	}
}

func TestGraphComplete_EmptyGraph(t *testing.T) {
	if !GraphComplete(nil, nil, nil) {
		t.Error("empty graph should be complete")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/queue/ -run "TestReadyNodes|TestGraphComplete" -v`
Expected: FAIL (undefined: ReadyNodes, GraphComplete)

**Step 3: Implement ReadyNodes and GraphComplete**

Add to `graph.go`:
```go
// ReadyNodes returns all pending tasks whose dependencies are fully satisfied
// and that are not currently active. Sorted by task ID.
func ReadyNodes(pending []*Task, active []*Task, done []*Task) []*Task {
	doneIDs := make(map[int]bool, len(done))
	for _, t := range done {
		doneIDs[t.ID] = true
	}
	activeIDs := make(map[int]bool, len(active))
	for _, t := range active {
		activeIDs[t.ID] = true
	}

	var ready []*Task
	for _, t := range pending {
		if activeIDs[t.ID] {
			continue
		}
		allDone := true
		for _, dep := range t.DependsOn {
			if !doneIDs[dep] {
				allDone = false
				break
			}
		}
		if allDone {
			ready = append(ready, t)
		}
	}

	sort.Slice(ready, func(i, j int) bool {
		return ready[i].ID < ready[j].ID
	})
	return ready
}

// GraphComplete returns true when there are no pending or active tasks.
func GraphComplete(pending []*Task, active []*Task, done []*Task) bool {
	return len(pending) == 0 && len(active) == 0
}
```

**Step 4: Run tests**

Run: `go test ./internal/queue/ -run "TestReadyNodes|TestGraphComplete" -v`
Expected: ALL PASS

**Step 5: Run full suite**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 6: Commit**

```
feat(graph): add ReadyNodes() and GraphComplete()
```

---

### Task 4: Convert `.doing` from file to directory

This is the largest task. Changes `Pop`, `Active`, `Complete`, `Requeue`, `NextID` to use `.doing/` directory. Also adds migration from old `.doing` file.

**Files:**
- Modify: `internal/queue/queue.go:14-17` (Queue struct — add scratchDir)
- Modify: `internal/queue/queue.go:22-30` (GetQueue — init scratchDir)
- Modify: `internal/queue/queue.go:133-167` (Pop)
- Modify: `internal/queue/queue.go:169-180` (Active)
- Modify: `internal/queue/queue.go:182-214` (Complete)
- Modify: `internal/queue/queue.go:216-235` (Requeue)
- Modify: `internal/queue/queue.go:301-346` (NextID)
- Modify: `internal/queue/queue_test.go` (update all .doing assertions)

**Step 1: Write tests for new .doing/ directory behavior**

Add to `queue_test.go`. Update `setupTestDir` first:
```go
func setupTestDir(t *testing.T) string {
	t.Helper()
	instance = nil
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "queue"), 0o755)
	os.MkdirAll(filepath.Join(dir, "queue", ".doing"), 0o755)
	os.MkdirAll(filepath.Join(dir, "queue", "done"), 0o755)
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.MkdirAll(filepath.Join(dir, "scratch"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "log.md"), []byte(""), 0o644)
	return dir
}
```

Add new tests:
```go
func TestPopIntoDoingDir(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})

	task, err := q.Pop()
	if err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if task.ID != 1 {
		t.Errorf("Pop ID = %d, want 1", task.ID)
	}

	// File should be in .doing/ directory
	matches, _ := filepath.Glob(filepath.Join(dir, "queue", ".doing", "001-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 file in .doing/, got %d", len(matches))
	}

	// Original should be gone
	pending, _ := filepath.Glob(filepath.Join(dir, "queue", "*.md"))
	if len(pending) != 0 {
		t.Errorf("original file still in queue/")
	}
}

func TestMultipleActiveTasks(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)
	q.Create("first", CreateOptions{})
	q.Create("second", CreateOptions{})

	q.Pop()  // pops task 1
	q.Pop()  // pops task 2 — now allowed!

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

	// Task 1 in done/, task 2 still active
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

func TestMigrateOldDoingFile(t *testing.T) {
	dir := setupTestDir(t)
	// Remove .doing/ dir and create old-style .doing file
	os.RemoveAll(filepath.Join(dir, "queue", ".doing"))

	task := &Task{ID: 1, Status: "doing", Title: "legacy task", Body: "# legacy task", Mode: "once"}
	os.WriteFile(filepath.Join(dir, "queue", ".doing"), task.Serialize(), 0o644)

	q := GetQueue(dir)
	q.MigrateDoingFile()

	tasks, err := q.Active()
	if err != nil {
		t.Fatalf("Active after migration: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 active after migration, got %d", len(tasks))
	}
	if tasks[0].ID != 1 {
		t.Errorf("migrated task ID = %d, want 1", tasks[0].ID)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/queue/ -run "TestPopIntoDoingDir|TestMultipleActive|TestCompleteByID|TestRequeueByID|TestMigrateOldDoingFile" -v`
Expected: FAIL

**Step 3: Implement .doing directory changes**

Rewrite `queue.go` methods:

**Queue struct** — add `scratchDir`:
```go
type Queue struct {
	queueDir   string
	scratchDir string
	logPath    string
}
```

**GetQueue** — init scratchDir + ensure .doing/ dir:
```go
func GetQueue(agentDir string) *Queue {
	if instance == nil {
		qDir := filepath.Join(agentDir, "queue")
		instance = &Queue{
			queueDir:   qDir,
			scratchDir: filepath.Join(agentDir, "scratch"),
			logPath:    filepath.Join(agentDir, "memory", "log.md"),
		}
		os.MkdirAll(filepath.Join(qDir, ".doing"), 0o755)
	}
	return instance
}
```

**Pop** — move into `.doing/` directory (no longer blocks on existing active):
```go
func (q *Queue) Pop() (*Task, error) {
	entries, err := filepath.Glob(filepath.Join(q.queueDir, "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)

	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		task, err := ParseTask(data)
		if err != nil || task.Status != "pending" {
			continue
		}

		task.Status = "doing"
		filename := filepath.Base(path)
		doingPath := filepath.Join(q.queueDir, ".doing", filename)
		if err := os.WriteFile(doingPath, task.Serialize(), 0o644); err != nil {
			return nil, fmt.Errorf("writing to .doing/: %w", err)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("removing original task file: %w", err)
		}
		return task, nil
	}

	return nil, fmt.Errorf("queue is empty")
}
```

**Active** — return all tasks in `.doing/`:
```go
func (q *Queue) Active() ([]*Task, error) {
	doingDir := filepath.Join(q.queueDir, ".doing")
	entries, err := filepath.Glob(filepath.Join(doingDir, "*.md"))
	if err != nil {
		return nil, err
	}

	var tasks []*Task
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		task, err := ParseTask(data)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})
	return tasks, nil
}
```

**CompleteByID** — complete a specific task:
```go
func (q *Queue) CompleteByID(id int, summary string) error {
	doingDir := filepath.Join(q.queueDir, ".doing")
	path, task, err := q.findActiveByID(id)
	if err != nil {
		return err
	}
	_ = doingDir // path already includes doingDir

	now := time.Now().UTC().Format(time.RFC3339)
	if summary == "" {
		summary = "completed"
	}
	entry := fmt.Sprintf("\n## %s — %s [task:%03d]\n%s\n", now, task.Title, task.ID, summary)
	if err := q.appendLog(entry); err != nil {
		return err
	}

	task.Status = "done"
	doneDir := filepath.Join(q.queueDir, "done")
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		return fmt.Errorf("creating done dir: %w", err)
	}
	slug := Slugify(task.Title)
	donePath := filepath.Join(doneDir, fmt.Sprintf("%03d-%s.md", task.ID, slug))
	if err := os.WriteFile(donePath, task.Serialize(), 0o644); err != nil {
		return fmt.Errorf("writing done task: %w", err)
	}

	return os.Remove(path)
}
```

**RequeueByID** — requeue a specific task:
```go
func (q *Queue) RequeueByID(id int) error {
	path, task, err := q.findActiveByID(id)
	if err != nil {
		return err
	}

	task.Status = "pending"
	slug := Slugify(task.Title)
	filename := fmt.Sprintf("%03d-%s.md", task.ID, slug)
	pendingPath := filepath.Join(q.queueDir, filename)

	if err := os.WriteFile(pendingPath, task.Serialize(), 0o644); err != nil {
		return err
	}
	return os.Remove(path)
}
```

**Helper — findActiveByID**:
```go
func (q *Queue) findActiveByID(id int) (string, *Task, error) {
	doingDir := filepath.Join(q.queueDir, ".doing")
	pattern := fmt.Sprintf("%03d-*.md", id)
	matches, err := filepath.Glob(filepath.Join(doingDir, pattern))
	if err != nil {
		return "", nil, err
	}
	if len(matches) == 0 {
		return "", nil, fmt.Errorf("no active task with ID %d", id)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return "", nil, err
	}
	task, err := ParseTask(data)
	if err != nil {
		return "", nil, err
	}
	return matches[0], task, nil
}
```

**MigrateDoingFile** — one-time migration:
```go
func (q *Queue) MigrateDoingFile() {
	oldPath := filepath.Join(q.queueDir, ".doing")
	info, err := os.Stat(oldPath)
	if err != nil || info.IsDir() {
		return // no old file or already a directory
	}

	data, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	task, err := ParseTask(data)
	if err != nil {
		return
	}

	doingDir := filepath.Join(q.queueDir, ".doing_dir_tmp")
	os.MkdirAll(doingDir, 0o755)

	slug := Slugify(task.Title)
	filename := fmt.Sprintf("%03d-%s.md", task.ID, slug)
	os.WriteFile(filepath.Join(doingDir, filename), data, 0o644)
	os.Remove(oldPath)
	os.Rename(doingDir, filepath.Join(q.queueDir, ".doing"))
}
```

**Keep old Complete/Requeue** as wrappers for backwards compat (used by `cmd/done.go`, `cmd/requeue.go`):
```go
func (q *Queue) Complete(summary string) error {
	tasks, err := q.Active()
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no active task")
	}
	if len(tasks) > 1 {
		return fmt.Errorf("multiple active tasks — use CompleteByID(id, summary)")
	}
	return q.CompleteByID(tasks[0].ID, summary)
}

func (q *Queue) Requeue() error {
	tasks, err := q.Active()
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no active task")
	}
	if len(tasks) > 1 {
		return fmt.Errorf("multiple active tasks — use RequeueByID(id)")
	}
	return q.RequeueByID(tasks[0].ID)
}
```

**Update NextID** — scan `.doing/` directory:
```go
// In NextID, replace the ".doing" file check with directory scan:

// Check .doing/ directory
doingEntries, _ := filepath.Glob(filepath.Join(q.queueDir, ".doing", "*.md"))
for _, path := range doingEntries {
	data, err := os.ReadFile(path)
	if err != nil {
		continue
	}
	t, err := ParseTask(data)
	if err != nil {
		continue
	}
	if t.ID > maxID {
		maxID = t.ID
	}
}
```

**Step 4: Update existing tests**

Fix `TestPop` — check `.doing/` directory instead of `.doing` file:
```go
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

	// .doing/ should contain the task
	matches, _ := filepath.Glob(filepath.Join(dir, "queue", ".doing", "001-*.md"))
	if len(matches) != 1 {
		t.Errorf("expected 1 file in .doing/, got %d", len(matches))
	}

	// Original file should be gone
	origMatches, _ := filepath.Glob(filepath.Join(dir, "queue", "001-*.md"))
	if len(origMatches) != 0 {
		t.Errorf("original file still exists after pop")
	}
}
```

Remove `TestPopRefusesWhenDoing` — multiple active is now allowed.

Fix `TestComplete` to use the new directory structure (single active still works via wrapper).

Fix `TestRequeue` similarly.

Fix `TestActiveTask` — `Active()` now returns `[]*Task`:
```go
func TestActiveTask(t *testing.T) {
	dir := setupTestDir(t)
	q := GetQueue(dir)

	// No active tasks
	tasks, err := q.Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if len(tasks) != 0 {
		t.Error("expected empty when no .doing/ contents")
	}

	// Pop creates active
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
```

**Step 5: Run all tests**

Run: `go test ./internal/queue/ -v`
Expected: ALL PASS

**Step 6: Commit**

```
feat(queue): convert .doing from file to directory for concurrent execution
```

---

### Task 5: Update all callers of `Active()`, `Complete()`, `Requeue()`

**Files:**
- Modify: `cmd/done.go` (Active returns []*Task)
- Modify: `cmd/requeue.go` (Active returns []*Task)
- Modify: `cmd/status.go` (Active returns []*Task)
- Modify: `cmd/run.go` (Active returns []*Task)
- Modify: `cmd/graph.go` (remove wrapper — Active returns []*Task directly)

**Step 1: Update cmd/done.go**

```go
var doneCmd = &cobra.Command{
	Use:   "done [summary]",
	Short: "Complete the active task",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		summary := strings.Join(args, " ")

		active, err := q.Active()
		if err != nil {
			return err
		}
		if len(active) == 0 {
			return fmt.Errorf("no active task")
		}
		if len(active) > 1 {
			fmt.Println("Multiple active tasks:")
			for _, t := range active {
				fmt.Printf("  %03d: %s\n", t.ID, t.Title)
			}
			return fmt.Errorf("specify task ID: cubit done <id> [summary]")
		}

		if err := q.CompleteByID(active[0].ID, summary); err != nil {
			return err
		}
		fmt.Printf("✓ %03d: %s\n", active[0].ID, active[0].Title)
		return nil
	},
}
```

**Step 2: Update cmd/requeue.go**

```go
var requeueCmd = &cobra.Command{
	Use:   "requeue",
	Short: "Return the active task to the queue",
	RunE: func(cmd *cobra.Command, args []string) error {
		active, err := q.Active()
		if err != nil {
			return err
		}
		if len(active) == 0 {
			return fmt.Errorf("no active task")
		}
		if len(active) > 1 {
			fmt.Println("Multiple active tasks:")
			for _, t := range active {
				fmt.Printf("  %03d: %s\n", t.ID, t.Title)
			}
			return fmt.Errorf("specify task ID: cubit requeue <id>")
		}

		if err := q.RequeueByID(active[0].ID); err != nil {
			return err
		}
		fmt.Printf("↩ %03d: %s\n", active[0].ID, active[0].Title)
		return nil
	},
}
```

**Step 3: Update cmd/status.go**

```go
// Replace the active task section:
active, err := q.Active()
if err != nil {
	return err
}
if len(active) > 0 {
	for _, t := range active {
		fmt.Printf("Active:  %03d — %s\n", t.ID, t.Title)
	}
} else {
	fmt.Println("Active:  (none)")
}
```

**Step 4: Update cmd/graph.go**

Remove the wrapper — Active() now returns []*Task directly:
```go
pending, err := q.List()
if err != nil {
	return err
}
active, err := q.Active()
if err != nil {
	return err
}
done, err := q.ListDone()
if err != nil {
	return err
}

nodes := queue.BuildGraph(pending, active, done)
```

**Step 5: Update cmd/run.go (minimal — will be rewritten in Task 9)**

```go
// Replace the active task check at line 30-43:
activeTasks, err := q.Active()
if err != nil {
	return err
}

var task *queue.Task
if len(activeTasks) > 0 {
	task = activeTasks[0]
} else {
	task, err = q.Pop()
	if err != nil {
		fmt.Println("Queue empty. Done.")
		return nil
	}
}
```

And replace `q.Complete(summarize(result))` with `q.CompleteByID(task.ID, summarize(result))`.

And replace `q.Requeue()` with `q.RequeueByID(task.ID)`.

**Step 6: Run all tests + build**

Run: `go build ./... && go test ./... -v`
Expected: ALL PASS

**Step 7: Commit**

```
refactor(cmd): update all callers for multi-active .doing/ directory
```

---

### Task 6: Add `PopReady()` and `PopAllReady()` to queue.go

DAG-aware popping — only pops tasks whose dependencies are all satisfied.

**Files:**
- Modify: `internal/queue/queue.go` (add PopReady, PopAllReady)
- Modify: `internal/queue/queue_test.go` (add tests)

**Step 1: Write failing tests**

```go
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

	// Task 3 should still be pending
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/queue/ -run "TestPopReady|TestPopAllReady" -v`
Expected: FAIL

**Step 3: Implement**

```go
// PopReady pops the lowest-ID pending task whose dependencies are all done.
func (q *Queue) PopReady() (*Task, error) {
	pending, err := q.List()
	if err != nil {
		return nil, err
	}
	active, err := q.Active()
	if err != nil {
		return nil, err
	}
	done, err := q.ListDone()
	if err != nil {
		return nil, err
	}

	ready := ReadyNodes(pending, active, done)
	if len(ready) == 0 {
		return nil, fmt.Errorf("no ready tasks")
	}

	return q.popByID(ready[0].ID)
}

// PopAllReady pops all pending tasks whose dependencies are all done.
func (q *Queue) PopAllReady() ([]*Task, error) {
	pending, err := q.List()
	if err != nil {
		return nil, err
	}
	active, err := q.Active()
	if err != nil {
		return nil, err
	}
	done, err := q.ListDone()
	if err != nil {
		return nil, err
	}

	ready := ReadyNodes(pending, active, done)
	var popped []*Task
	for _, t := range ready {
		task, err := q.popByID(t.ID)
		if err != nil {
			return popped, err
		}
		popped = append(popped, task)
	}
	return popped, nil
}

// popByID moves a specific pending task into .doing/.
func (q *Queue) popByID(id int) (*Task, error) {
	pattern := fmt.Sprintf("%03d-*.md", id)
	matches, err := filepath.Glob(filepath.Join(q.queueDir, pattern))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("task %03d not found in queue", id)
	}

	path := matches[0]
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	task, err := ParseTask(data)
	if err != nil {
		return nil, err
	}

	task.Status = "doing"
	filename := filepath.Base(path)
	doingPath := filepath.Join(q.queueDir, ".doing", filename)
	if err := os.WriteFile(doingPath, task.Serialize(), 0o644); err != nil {
		return nil, fmt.Errorf("writing to .doing/: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("removing original: %w", err)
	}
	return task, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/queue/ -run "TestPopReady|TestPopAllReady" -v`
Expected: ALL PASS

**Step 5: Run full suite**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 6: Commit**

```
feat(queue): add PopReady() and PopAllReady() for DAG-aware popping
```

---

### Task 7: Update `cubit do` — ready-only + `--all` flag

**Files:**
- Modify: `cmd/do.go` (rewrite command)
- Modify: `cmd/root.go:67-68` (register --all flag)

**Step 1: Rewrite cmd/do.go**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var doCmd = &cobra.Command{
	Use:   "do",
	Short: "Pop the next ready task (or all with --all)",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")

		if all {
			tasks, err := q.PopAllReady()
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Println("No ready tasks.")
				return nil
			}
			for _, t := range tasks {
				fmt.Printf("▶ %03d: %s\n", t.ID, t.Title)
			}
			fmt.Printf("(%d tasks active)\n", len(tasks))
			return nil
		}

		task, err := q.PopReady()
		if err != nil {
			return err
		}
		fmt.Printf("▶ %03d: %s\n", task.ID, task.Title)
		return nil
	},
}
```

**Step 2: Register flag in cmd/root.go**

Add after the `rootCmd.AddCommand(doCmd)` line:
```go
doCmd.Flags().Bool("all", false, "Pop all ready tasks at once")
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: clean build

**Step 4: Commit**

```
feat(do): only pop ready nodes, add --all flag
```

---

### Task 8: Add upstream output injection to brief.Build()

**Files:**
- Modify: `internal/brief/brief.go:19-55` (Build gains upstream param)
- Create: `internal/brief/brief_test.go` (test upstream injection)

**Step 1: Write failing test**

Create `internal/brief/brief_test.go`:
```go
package brief

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupBriefTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "identity"), 0o755)
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.MkdirAll(filepath.Join(dir, "queue", ".doing"), 0o755)
	os.MkdirAll(filepath.Join(dir, "scratch"), 0o755)
	os.WriteFile(filepath.Join(dir, "identity", "FLUCTLIGHT.md"), []byte("I am an agent"), 0o644)
	return dir
}

func TestBuildWithUpstream_InjectsOutputPaths(t *testing.T) {
	dir := setupBriefTestDir(t)

	// Create upstream output files
	os.WriteFile(filepath.Join(dir, "scratch", "001-output.md"), []byte("result 1"), 0o644)
	os.WriteFile(filepath.Join(dir, "scratch", "002-output.md"), []byte("result 2"), 0o644)

	result := BuildWithUpstream(dir, []int{1, 2})

	if !strings.Contains(result, "## Upstream Results") {
		t.Error("missing Upstream Results section")
	}
	if !strings.Contains(result, "scratch/001-output.md") {
		t.Error("missing output path for task 1")
	}
	if !strings.Contains(result, "scratch/002-output.md") {
		t.Error("missing output path for task 2")
	}
}

func TestBuildWithUpstream_SkipsMissingOutputs(t *testing.T) {
	dir := setupBriefTestDir(t)

	// Only task 1 has output
	os.WriteFile(filepath.Join(dir, "scratch", "001-output.md"), []byte("result 1"), 0o644)

	result := BuildWithUpstream(dir, []int{1, 2})

	if !strings.Contains(result, "001-output.md") {
		t.Error("should include existing output")
	}
	if strings.Contains(result, "002-output.md") {
		t.Error("should not include missing output")
	}
}

func TestBuildWithUpstream_NoUpstream(t *testing.T) {
	dir := setupBriefTestDir(t)

	result := BuildWithUpstream(dir, nil)

	if strings.Contains(result, "Upstream Results") {
		t.Error("should not have Upstream Results with no upstream IDs")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/brief/ -run TestBuildWithUpstream -v`
Expected: FAIL (undefined: BuildWithUpstream)

**Step 3: Implement BuildWithUpstream**

Add to `brief.go`:
```go
// BuildWithUpstream builds the session brief and appends upstream output paths
// for fan-in nodes. upstreamIDs are task IDs whose outputs should be referenced.
func BuildWithUpstream(agentDir string, upstreamIDs []int) string {
	base := Build(agentDir)

	if len(upstreamIDs) == 0 {
		return base
	}

	var paths []string
	for _, id := range upstreamIDs {
		filename := fmt.Sprintf("%03d-output.md", id)
		relPath := filepath.Join("scratch", filename)
		absPath := filepath.Join(agentDir, relPath)
		if _, err := os.Stat(absPath); err == nil {
			paths = append(paths, "- "+relPath)
		}
	}

	if len(paths) == 0 {
		return base
	}

	upstream := "## Upstream Results\n" + strings.Join(paths, "\n")
	return base + "\n\n---\n\n" + upstream
}
```

**Step 4: Run tests**

Run: `go test ./internal/brief/ -run TestBuildWithUpstream -v`
Expected: ALL PASS

**Step 5: Commit**

```
feat(brief): add BuildWithUpstream() for fan-in output injection
```

---

### Task 9: Build the concurrent executor

**Files:**
- Create: `internal/queue/executor.go`
- Create: `internal/queue/executor_test.go`

**Step 1: Write tests for executor helpers**

Create `internal/queue/executor_test.go`:
```go
package queue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTaskOutput(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	WriteTaskOutput(scratchDir, 1, "hello world")

	path := filepath.Join(scratchDir, "001-output.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("output = %q, want %q", string(data), "hello world")
	}
}

func TestWriteTaskOutput_FailureNote(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	WriteTaskOutput(scratchDir, 5, "")

	path := filepath.Join(scratchDir, "005-output.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "" {
		t.Errorf("expected empty output for failure, got %q", string(data))
	}
}

func TestTaskResult_Fields(t *testing.T) {
	r := TaskResult{
		TaskID: 1,
		Output: "some output",
		Err:    nil,
	}
	if r.TaskID != 1 {
		t.Errorf("TaskID = %d, want 1", r.TaskID)
	}
	if r.Failed() {
		t.Error("expected not failed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/queue/ -run "TestWriteTaskOutput|TestTaskResult" -v`
Expected: FAIL

**Step 3: Implement executor.go**

Create `internal/queue/executor.go`:
```go
package queue

import (
	"fmt"
	"os"
	"path/filepath"
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
	msg := "deadlock: no tasks can make progress\n"
	for _, t := range e.Stuck {
		msg += fmt.Sprintf("  %03d: %s (waiting on %v)\n", t.ID, t.Title, t.DependsOn)
	}
	return msg
}
```

**Step 4: Run tests**

Run: `go test ./internal/queue/ -run "TestWriteTaskOutput|TestTaskResult" -v`
Expected: ALL PASS

**Step 5: Commit**

```
feat(queue): add executor types and output helpers
```

---

### Task 10: Rewrite `cmd/run.go` with concurrent executor

**Files:**
- Modify: `cmd/run.go` (full rewrite)
- Modify: `cmd/root.go:86-90` (add --max-parallel flag)

**Step 1: Register --max-parallel flag in root.go**

Add to the run flags section:
```go
runCmd.Flags().Int("max-parallel", 0, "Max concurrent tasks (0 = NumCPU*4)")
```

**Step 2: Rewrite cmd/run.go**

```go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/SeanoChang/cubit/internal/claude"
	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Resolve the task DAG: fan-out ready tasks, fan-in at dependencies",
	Long:  "Concurrent DAG executor. Finds all ready tasks, runs them in parallel (up to --max-parallel), waits for completions to unlock dependents. Stops when graph is fully resolved or deadlocked.",
	RunE: func(cmd *cobra.Command, args []string) error {
		once, _ := cmd.Flags().GetBool("once")
		cooldown, _ := cmd.Flags().GetDuration("cooldown")
		noMemory, _ := cmd.Flags().GetBool("no-memory")
		maxParallel, _ := cmd.Flags().GetInt("max-parallel")

		if maxParallel <= 0 {
			maxParallel = cfg.Claude.MaxParallel
		}
		if maxParallel <= 0 {
			maxParallel = runtime.NumCPU() * 4
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		sem := make(chan struct{}, maxParallel)
		doneCh := make(chan queue.TaskResult, 64)
		running := 0

		// Track graph state in main goroutine
		dispatched := make(map[int]bool)

		fmt.Printf("Starting DAG executor (max-parallel: %d)\n", maxParallel)

		for {
			// Check for cancellation
			select {
			case <-ctx.Done():
				fmt.Println("\nInterrupted. Waiting for running tasks...")
				for running > 0 {
					result := <-doneCh
					running--
					handleResult(result, noMemory, cooldown)
				}
				return nil
			default:
			}

			// Scan for ready nodes
			pending, err := q.List()
			if err != nil {
				return err
			}
			active, err := q.Active()
			if err != nil {
				return err
			}
			doneList, err := q.ListDone()
			if err != nil {
				return err
			}

			ready := queue.ReadyNodes(pending, active, doneList)

			// Filter out already-dispatched tasks
			var toDispatch []*queue.Task
			for _, t := range ready {
				if !dispatched[t.ID] {
					toDispatch = append(toDispatch, t)
				}
			}

			// Launch ready tasks
			for _, task := range toDispatch {
				// Pop into .doing/
				popped, err := q.PopReady()
				if err != nil {
					break
				}

				dispatched[popped.ID] = true

				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					// Cancelled while waiting for semaphore — requeue
					q.RequeueByID(popped.ID)
					goto drain
				}

				running++
				fmt.Printf("▶ %03d: %s\n", popped.ID, popped.Title)

				go func(t *queue.Task) {
					defer func() { <-sem }()
					result := executeWithRetry(ctx, t, 3)
					doneCh <- result
				}(popped)

				if once {
					// In --once mode, only dispatch one task
					break
				}
			}

			// Terminal condition
			if running == 0 {
				if queue.GraphComplete(pending, active, doneList) {
					fmt.Println("Graph resolved. Done.")
					return nil
				}
				// Deadlock
				var stuck []*queue.Task
				for _, t := range pending {
					stuck = append(stuck, t)
				}
				return &queue.DeadlockError{Stuck: stuck}
			}

			// Wait for exactly one completion
			result := <-doneCh
			running--
			delete(dispatched, result.TaskID)
			handleResult(result, noMemory, cooldown)

			if once {
				// Drain remaining and exit
				goto drain
			}

			continue

		drain:
			for running > 0 {
				r := <-doneCh
				running--
				delete(dispatched, r.TaskID)
				handleResult(r, noMemory, cooldown)
			}
			return nil
		}
	},
}

func executeWithRetry(ctx context.Context, task *queue.Task, maxRetries int) queue.TaskResult {
	agentDir := cfg.AgentDir()
	scratchDir := cfg.AgentDir() + "/scratch"

	// Build brief with upstream output paths for fan-in nodes
	injection := brief.BuildWithUpstream(agentDir, task.DependsOn)
	full := injection + "\n\n---\n\nExecute the active task."

	// Resolve model: task override → config default
	model := task.Model
	if model == "" {
		model = cfg.Claude.Model
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return queue.TaskResult{
				TaskID: task.ID,
				Err:    ctx.Err(),
				Model:  model,
			}
		default:
		}

		if attempt > 0 {
			fmt.Fprintf(os.Stderr, "  %03d: retry %d/%d\n", task.ID, attempt, maxRetries)
		}

		output, err := claude.Prompt(full, model)
		if err != nil {
			lastErr = err
			continue
		}

		// Success
		queue.WriteTaskOutput(scratchDir, task.ID, output)
		return queue.TaskResult{
			TaskID:  task.ID,
			Output:  output,
			Summary: summarize(output),
			Model:   model,
		}
	}

	// All retries exhausted — write empty output, return failure
	queue.WriteTaskOutput(scratchDir, task.ID, "")
	return queue.TaskResult{
		TaskID:  task.ID,
		Summary: fmt.Sprintf("FAILED after %d attempts: %v", maxRetries+1, lastErr),
		Err:     lastErr,
		Model:   model,
	}
}

func handleResult(result queue.TaskResult, noMemory bool, cooldown time.Duration) {
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "✗ %03d: %s\n", result.TaskID, result.Err)
		// Mark done with failure note (don't requeue — let the model triage later)
		q.CompleteByID(result.TaskID, result.Summary)
		return
	}

	fmt.Printf("\n%s\n\n", result.Output)

	if err := q.CompleteByID(result.TaskID, result.Summary); err != nil {
		fmt.Fprintf(os.Stderr, "  complete error %03d: %v\n", result.TaskID, err)
		return
	}
	fmt.Printf("✓ %03d\n", result.TaskID)

	// Memory pass (sequential, in main goroutine)
	if !noMemory {
		if err := brief.RunMemoryPass(cfg.AgentDir(), result.Output, cfg.Claude.MemoryModel); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: memory pass failed: %v\n", err)
		}
	}
}

func sleepOrCancel(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		fmt.Println("\nInterrupted. Shutting down.")
		return false
	}
}
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: clean build

**Step 4: Commit**

```
feat(run): concurrent DAG executor with fan-out/fan-in
```

---

### Task 11: Update brief.go Sections() for .doing/ directory

The `Sections()` function (used by `cubit brief` and `cubit status`) reads `queue/.doing` as a file. Update it to read all files in `queue/.doing/`.

**Files:**
- Modify: `internal/brief/brief.go:19-55` (Build — update .doing entry)
- Modify: `internal/brief/brief.go:63-85` (Sections — update .doing entry)

**Step 1: Update Build() to read .doing/ directory**

Replace the hardcoded `{"queue/.doing", "## Active Task\n"}` entry.
Instead, after the fixed entries, dynamically read `.doing/`:

```go
func Build(agentDir string) string {
	var parts []string

	entries := []struct {
		rel    string
		prefix string
	}{
		{"identity/FLUCTLIGHT.md", ""},
		{"USER.md", ""},
		{"GOALS.md", ""},
		{"memory/brief.md", ""},
	}

	for _, e := range entries {
		content := readFile(filepath.Join(agentDir, e.rel))
		if content == "" {
			continue
		}
		if e.rel == "memory/brief.md" {
			if tok := EstimateTokens(content); tok > 30000 {
				log.Printf("warning: memory/brief.md is ~%d tokens (budget 30k)", tok)
			}
		}
		if e.prefix != "" {
			content = e.prefix + content
		}
		parts = append(parts, content)
	}

	// Active tasks from .doing/ directory
	doingDir := filepath.Join(agentDir, "queue", ".doing")
	doingFiles, _ := filepath.Glob(filepath.Join(doingDir, "*.md"))
	if len(doingFiles) > 0 {
		var taskParts []string
		for _, f := range doingFiles {
			content := readFile(f)
			if content != "" {
				taskParts = append(taskParts, content)
			}
		}
		if len(taskParts) > 0 {
			active := "## Active Tasks\n" + strings.Join(taskParts, "\n\n")
			parts = append(parts, active)
		}
	}

	// Current plan
	if plan := readFile(filepath.Join(agentDir, "scratch", "plan.md")); plan != "" {
		parts = append(parts, "## Current Plan\n"+plan)
	}

	return strings.Join(parts, "\n\n---\n\n")
}
```

**Step 2: Update Sections() similarly**

```go
func Sections(agentDir string) []Section {
	entries := []struct {
		rel   string
		label string
	}{
		{"identity/FLUCTLIGHT.md", "FLUCTLIGHT"},
		{"USER.md", "USER"},
		{"GOALS.md", "GOALS"},
		{"memory/brief.md", "Brief"},
	}

	var sections []Section
	for _, e := range entries {
		content := readFile(filepath.Join(agentDir, e.rel))
		sections = append(sections, Section{
			Label:   e.label,
			Content: content,
		})
	}

	// Active tasks
	doingDir := filepath.Join(agentDir, "queue", ".doing")
	doingFiles, _ := filepath.Glob(filepath.Join(doingDir, "*.md"))
	var activeContent string
	for _, f := range doingFiles {
		content := readFile(f)
		if content != "" {
			if activeContent != "" {
				activeContent += "\n\n"
			}
			activeContent += content
		}
	}
	sections = append(sections, Section{Label: "Active Tasks", Content: activeContent})

	// Current plan
	sections = append(sections, Section{
		Label:   "Current Plan",
		Content: readFile(filepath.Join(agentDir, "scratch", "plan.md")),
	})

	return sections
}
```

**Step 3: Verify build + tests**

Run: `go build ./... && go test ./... -v`
Expected: ALL PASS

**Step 4: Commit**

```
feat(brief): read active tasks from .doing/ directory
```

---

### Task 12: Call MigrateDoingFile on startup

**Files:**
- Modify: `cmd/root.go:23-31` (PersistentPreRunE)

**Step 1: Add migration call**

In `PersistentPreRunE`, after `q = queue.GetQueue(cfg.AgentDir())`:
```go
q.MigrateDoingFile()
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: clean build

**Step 3: Commit**

```
feat(root): migrate old .doing file to .doing/ directory on startup
```

---

### Task 13: Manual integration test

**Step 1: Build**

Run: `go build -o cubit .`

**Step 2: Create test tasks**

```bash
./cubit todo "task A — no deps"
./cubit todo "task B — no deps"
./cubit todo "task C — depends on A and B" --depends-on 1,2
./cubit graph
```

Expected graph:
```
  001 [once] task A                              ✓ ready
  002 [once] task B                              ✓ ready
  003 [once] task C — depends on A ...           ⏳ waiting on [001, 002]
```

**Step 3: Test cubit do --all**

```bash
./cubit do --all
```

Expected: tasks 1 and 2 popped, task 3 still pending.

**Step 4: Test cubit run (with actual Claude calls)**

```bash
./cubit run --max-parallel 2
```

Expected: tasks 1 and 2 run in parallel, task 3 starts after both complete.

**Step 5: Commit**

```
feat: M9 concurrent DAG executor complete
```
