# M8: cubit graph Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `cubit graph` command that prints the full task DAG with status (done/active/waiting/ready), detect circular deps, and add a per-task `model` field to `Task` with `--model` flag on `cubit todo` (falls back to `cfg.Claude.Model`).

**Architecture:** Add a `done/` subdirectory under `queue/` to persist completed tasks (currently they are deleted). Add `internal/queue/graph.go` for DAG logic (BuildGraph, DetectCycle, ValidateDependencies). Add `model` field to `Task` struct + `--model` flag wired through `cubit todo`. Add `cmd/graph.go` for the new command. Wire cycle validation into `cubit todo` at creation time.

**Tech Stack:** Go stdlib only. Cobra for command. DFS coloring for cycle detection. Config default: `cfg.Claude.Model` (`claude-opus-4-6`).

---

## Context (read before starting)

Current state of the queue package:
- `internal/queue/task.go` — `Task` struct already has `DependsOn []int`, `Mode`, `Program`, `Goal`, `MaxIterations`, `Branch`
- `internal/queue/queue.go` — `Complete()` currently deletes `.doing` (no persistence of done tasks)
- `internal/scaffold/scaffold.go` — creates `queue/` dir but not `queue/done/`
- `cmd/todo.go` — already accepts `--depends-on` flag but does NOT validate for cycles

Node status logic:
- `done` — file exists in `queue/done/`
- `active` — is the current `queue/.doing`
- `ready` — in `queue/*.md`, all `depends_on` IDs are in done set
- `waiting` — in `queue/*.md`, at least one `depends_on` ID is not in done set

Graph output format:
```
001 [loop] arch sweep            → DONE
002 [loop] ablation study        ← ACTIVE
003 [once] write report           ⏳ waiting on [001]
004 [once] publish                ⏳ waiting on [002, 003]
005 [once] deploy                 ✓ ready
```

---

## Task 1: Persist done tasks in `queue/done/`

**Files:**
- Modify: `internal/scaffold/scaffold.go`
- Modify: `internal/queue/queue.go`
- Modify: `internal/queue/queue_test.go`

### Step 1: Add `queue/done/` to scaffold

In `internal/scaffold/scaffold.go`, add the done directory to the `dirs` slice:

```go
dirs := []string{
    filepath.Join(agentDir, "identity"),
    filepath.Join(agentDir, "queue"),
    filepath.Join(agentDir, "queue", "done"),
    filepath.Join(agentDir, "scratch"),
    filepath.Join(agentDir, "memory", "sessions"),
}
```

### Step 2: Modify `Complete()` in `internal/queue/queue.go`

Replace the final `os.Remove` in `Complete()` with a move to `done/`:

```go
func (q *Queue) Complete(summary string) error {
    task, err := q.Active()
    if err != nil {
        return err
    }
    if task == nil {
        return fmt.Errorf("no active task")
    }

    now := time.Now().UTC().Format(time.RFC3339)
    if summary == "" {
        summary = "completed"
    }
    entry := fmt.Sprintf("\n## %s — %s [task:%03d]\n%s\n", now, task.Title, task.ID, summary)

    if err := q.appendLog(entry); err != nil {
        return err
    }

    // Move .doing to done/ (persist for graph)
    task.Status = "done"
    slug := Slugify(task.Title)
    donePath := filepath.Join(q.queueDir, "done", fmt.Sprintf("%03d-%s.md", task.ID, slug))
    if err := os.MkdirAll(filepath.Dir(donePath), 0o755); err != nil {
        return err
    }
    if err := os.WriteFile(donePath, task.Serialize(), 0o644); err != nil {
        return err
    }
    return os.Remove(filepath.Join(q.queueDir, ".doing"))
}
```

### Step 3: Add `ListDone()` to `internal/queue/queue.go`

```go
// ListDone returns all completed tasks sorted by ID.
func (q *Queue) ListDone() ([]*Task, error) {
    entries, err := filepath.Glob(filepath.Join(q.queueDir, "done", "*.md"))
    if err != nil {
        return nil, err
    }
    var tasks []*Task
    for _, path := range entries {
        data, err := os.ReadFile(path)
        if err != nil {
            return nil, err
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

### Step 4: Update `TestComplete` in `internal/queue/queue_test.go`

The existing `TestComplete` checks that `.doing` is removed. Add a check that the done file was created:

```go
func TestComplete(t *testing.T) {
    dir := setupTestDir(t)
    // Need done/ subdir for Complete to work
    os.MkdirAll(filepath.Join(dir, "queue", "done"), 0o755)
    q := GetQueue(dir)
    q.Create("first", CreateOptions{})
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

    // done/ should have a file for task 001
    pattern := filepath.Join(dir, "queue", "done", "001-*.md")
    matches, _ := filepath.Glob(pattern)
    if len(matches) != 1 {
        t.Errorf("expected 1 done file, got %d", len(matches))
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
```

### Step 5: Add `TestListDone` to `internal/queue/queue_test.go`

```go
func TestListDone(t *testing.T) {
    dir := setupTestDir(t)
    os.MkdirAll(filepath.Join(dir, "queue", "done"), 0o755)
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
```

### Step 6: Also update `setupTestDir` to create `queue/done/`

```go
func setupTestDir(t *testing.T) string {
    t.Helper()
    instance = nil
    dir := t.TempDir()
    os.MkdirAll(filepath.Join(dir, "queue"), 0o755)
    os.MkdirAll(filepath.Join(dir, "queue", "done"), 0o755)
    os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
    os.WriteFile(filepath.Join(dir, "memory", "log.md"), []byte(""), 0o644)
    return dir
}
```

### Step 7: Run tests

```bash
go test ./internal/queue/... -v -run TestComplete
go test ./internal/queue/... -v -run TestListDone
```

Expected: both PASS.

### Step 8: Run full test suite

```bash
go test ./...
```

Expected: all tests PASS.

### Step 9: Commit

```bash
git add internal/scaffold/scaffold.go internal/queue/queue.go internal/queue/queue_test.go
git commit -m "feat: persist done tasks in queue/done/ for graph visibility"
```

---

## Task 2: Graph logic — `BuildGraph()` and node status

**Files:**
- Create: `internal/queue/graph.go`
- Create: `internal/queue/graph_test.go`

### Step 1: Write the failing tests in `internal/queue/graph_test.go`

```go
package queue

import (
    "testing"
)

func TestBuildGraph_AllStatuses(t *testing.T) {
    // 001 is done, 002 is active, 003 depends on 001 (ready), 004 depends on 002 (waiting)
    done := []*Task{
        {ID: 1, Title: "arch sweep", Mode: "loop", Status: "done", DependsOn: []int{}},
    }
    active := &Task{ID: 2, Title: "ablation", Mode: "loop", Status: "doing", DependsOn: []int{1}}
    pending := []*Task{
        {ID: 3, Title: "write report", Mode: "once", Status: "pending", DependsOn: []int{1}},
        {ID: 4, Title: "publish", Mode: "once", Status: "pending", DependsOn: []int{2, 3}},
        {ID: 5, Title: "deploy", Mode: "once", Status: "pending", DependsOn: []int{}},
    }

    nodes := BuildGraph(pending, active, done)

    want := map[int]NodeStatus{
        1: StatusDone,
        2: StatusActive,
        3: StatusReady,   // dep 001 is done
        4: StatusWaiting, // dep 002 is active (not done)
        5: StatusReady,   // no deps
    }
    for _, n := range nodes {
        if n.Status != want[n.Task.ID] {
            t.Errorf("node %d: status = %q, want %q", n.Task.ID, n.Status, want[n.Task.ID])
        }
    }
}

func TestBuildGraph_NoActive(t *testing.T) {
    pending := []*Task{
        {ID: 1, Title: "first", Mode: "once", Status: "pending", DependsOn: []int{}},
    }
    nodes := BuildGraph(pending, nil, nil)
    if len(nodes) != 1 {
        t.Fatalf("want 1 node, got %d", len(nodes))
    }
    if nodes[0].Status != StatusReady {
        t.Errorf("status = %q, want ready", nodes[0].Status)
    }
}

func TestBuildGraph_OrderByID(t *testing.T) {
    pending := []*Task{
        {ID: 3, Title: "c", Mode: "once", Status: "pending"},
        {ID: 1, Title: "a", Mode: "once", Status: "pending"},
        {ID: 2, Title: "b", Mode: "once", Status: "pending"},
    }
    nodes := BuildGraph(pending, nil, nil)
    for i, n := range nodes {
        if n.Task.ID != i+1 {
            t.Errorf("node[%d].ID = %d, want %d", i, n.Task.ID, i+1)
        }
    }
}
```

### Step 2: Run to confirm FAIL

```bash
go test ./internal/queue/... -v -run TestBuildGraph
```

Expected: FAIL with "undefined: BuildGraph"

### Step 3: Implement `internal/queue/graph.go`

```go
package queue

import "sort"

// NodeStatus describes where a task sits in the graph.
type NodeStatus string

const (
    StatusDone    NodeStatus = "done"
    StatusActive  NodeStatus = "active"
    StatusReady   NodeStatus = "ready"
    StatusWaiting NodeStatus = "waiting"
)

// GraphNode pairs a task with its computed status.
type GraphNode struct {
    Task   *Task
    Status NodeStatus
}

// BuildGraph assembles all tasks into an ordered slice of GraphNodes.
// pending: tasks in queue/*.md
// active: task in queue/.doing (nil if none)
// done: tasks in queue/done/*.md (nil if none)
func BuildGraph(pending []*Task, active *Task, done []*Task) []*GraphNode {
    doneIDs := make(map[int]bool)
    for _, t := range done {
        doneIDs[t.ID] = true
    }

    var nodes []*GraphNode

    // Done nodes
    for _, t := range done {
        nodes = append(nodes, &GraphNode{Task: t, Status: StatusDone})
    }

    // Active node
    if active != nil {
        nodes = append(nodes, &GraphNode{Task: active, Status: StatusActive})
    }

    // Pending nodes: ready if all deps done, waiting otherwise
    for _, t := range pending {
        status := StatusReady
        for _, dep := range t.DependsOn {
            if !doneIDs[dep] {
                status = StatusWaiting
                break
            }
        }
        nodes = append(nodes, &GraphNode{Task: t, Status: status})
    }

    // Sort by ID for stable output
    sort.Slice(nodes, func(i, j int) bool {
        return nodes[i].Task.ID < nodes[j].Task.ID
    })

    return nodes
}
```

### Step 4: Run tests to confirm PASS

```bash
go test ./internal/queue/... -v -run TestBuildGraph
```

Expected: all PASS.

### Step 5: Commit

```bash
git add internal/queue/graph.go internal/queue/graph_test.go
git commit -m "feat: add BuildGraph for DAG node status computation"
```

---

## Task 3: Cycle detection — `DetectCycle()` and `ValidateDependencies()`

**Files:**
- Modify: `internal/queue/graph.go`
- Modify: `internal/queue/graph_test.go`

### Step 1: Write failing cycle detection tests

Add to `internal/queue/graph_test.go`:

```go
func TestDetectCycle_NoCycle(t *testing.T) {
    // 1 → 2 → 3 (linear chain, no cycle)
    nodes := []*GraphNode{
        {Task: &Task{ID: 1, DependsOn: []int{}}},
        {Task: &Task{ID: 2, DependsOn: []int{1}}},
        {Task: &Task{ID: 3, DependsOn: []int{2}}},
    }
    if err := DetectCycle(nodes); err != nil {
        t.Errorf("expected no cycle, got: %v", err)
    }
}

func TestDetectCycle_DirectCycle(t *testing.T) {
    // 1 → 2 → 1 (direct cycle)
    nodes := []*GraphNode{
        {Task: &Task{ID: 1, DependsOn: []int{2}}},
        {Task: &Task{ID: 2, DependsOn: []int{1}}},
    }
    if err := DetectCycle(nodes); err == nil {
        t.Error("expected cycle error, got nil")
    }
}

func TestDetectCycle_IndirectCycle(t *testing.T) {
    // 1 → 2 → 3 → 1
    nodes := []*GraphNode{
        {Task: &Task{ID: 1, DependsOn: []int{3}}},
        {Task: &Task{ID: 2, DependsOn: []int{1}}},
        {Task: &Task{ID: 3, DependsOn: []int{2}}},
    }
    if err := DetectCycle(nodes); err == nil {
        t.Error("expected cycle error, got nil")
    }
}

func TestDetectCycle_DiamondNoCycle(t *testing.T) {
    // 1 → 2 → 4, 1 → 3 → 4 (diamond, valid DAG)
    nodes := []*GraphNode{
        {Task: &Task{ID: 1, DependsOn: []int{}}},
        {Task: &Task{ID: 2, DependsOn: []int{1}}},
        {Task: &Task{ID: 3, DependsOn: []int{1}}},
        {Task: &Task{ID: 4, DependsOn: []int{2, 3}}},
    }
    if err := DetectCycle(nodes); err != nil {
        t.Errorf("expected no cycle in diamond, got: %v", err)
    }
}
```

### Step 2: Run to confirm FAIL

```bash
go test ./internal/queue/... -v -run TestDetectCycle
```

Expected: FAIL with "undefined: DetectCycle"

### Step 3: Implement `DetectCycle()` in `internal/queue/graph.go`

Add after `BuildGraph`:

```go
// DetectCycle checks for circular dependencies using DFS coloring.
// Returns an error describing the cycle if one is found.
func DetectCycle(nodes []*GraphNode) error {
    // Build adjacency: id → list of dep IDs (edges go from dependent → dependency)
    // For cycle detection we need to traverse "depends_on" edges
    adj := make(map[int][]int)
    for _, n := range nodes {
        adj[n.Task.ID] = n.Task.DependsOn
    }

    const (
        white = 0 // unvisited
        gray  = 1 // in current DFS path
        black = 2 // fully processed
    )
    color := make(map[int]int)

    var visit func(id int) error
    visit = func(id int) error {
        color[id] = gray
        for _, dep := range adj[id] {
            if color[dep] == gray {
                return fmt.Errorf("circular dependency detected: %d → %d", id, dep)
            }
            if color[dep] == white {
                if err := visit(dep); err != nil {
                    return err
                }
            }
        }
        color[id] = black
        return nil
    }

    for _, n := range nodes {
        if color[n.Task.ID] == white {
            if err := visit(n.Task.ID); err != nil {
                return err
            }
        }
    }
    return nil
}
```

Add `"fmt"` to the import in `graph.go`:

```go
import (
    "fmt"
    "sort"
)
```

### Step 4: Run tests to confirm PASS

```bash
go test ./internal/queue/... -v -run TestDetectCycle
```

Expected: all PASS.

### Step 5: Add `ValidateDependencies()` to `internal/queue/queue.go`

This is called by `cubit todo` before creating a task to reject cycles early.

```go
// ValidateDependencies checks whether adding a task with the given ID and deps
// would introduce a cycle. Call before Create(). Returns an error if cyclic.
func (q *Queue) ValidateDependencies(newID int, deps []int) error {
    if len(deps) == 0 {
        return nil
    }

    pending, err := q.List()
    if err != nil {
        return err
    }
    done, err := q.ListDone()
    if err != nil {
        return err
    }
    active, err := q.Active()
    if err != nil {
        return err
    }

    // Build graph nodes from existing tasks + hypothetical new task
    newTask := &Task{ID: newID, DependsOn: deps}
    pending = append(pending, newTask)
    nodes := BuildGraph(pending, active, done)

    return DetectCycle(nodes)
}
```

### Step 6: Write test for `ValidateDependencies` in `queue_test.go`

```go
func TestValidateDependencies_NoCycle(t *testing.T) {
    dir := setupTestDir(t)
    q := GetQueue(dir)
    q.Create("first", CreateOptions{})

    // task 2 depends on task 1 — valid
    err := q.ValidateDependencies(2, []int{1})
    if err != nil {
        t.Errorf("expected no error, got: %v", err)
    }
}

func TestValidateDependencies_Cycle(t *testing.T) {
    dir := setupTestDir(t)
    q := GetQueue(dir)
    // Create task 1 that depends on task 2 (which doesn't exist yet but will be ID 2)
    q.Create("first", CreateOptions{DependsOn: []int{2}})

    // Now try to create task 2 depending on task 1 — would be a cycle
    err := q.ValidateDependencies(2, []int{1})
    if err == nil {
        t.Error("expected cycle error, got nil")
    }
}
```

### Step 7: Run all queue tests

```bash
go test ./internal/queue/... -v
```

Expected: all PASS.

### Step 8: Commit

```bash
git add internal/queue/graph.go internal/queue/graph_test.go internal/queue/queue.go internal/queue/queue_test.go
git commit -m "feat: add cycle detection and ValidateDependencies to queue"
```

---

## Task 4: Wire cycle validation into `cubit todo`

**Files:**
- Modify: `cmd/todo.go`

### Step 1: Update `cmd/todo.go` to call `ValidateDependencies` before `Create`

In the `RunE` function, after computing `dependsOn` and before calling `q.Create`, add:

```go
if len(dependsOn) > 0 {
    nextID := q.NextID()
    if err := q.ValidateDependencies(nextID, dependsOn); err != nil {
        return fmt.Errorf("dependency validation: %w", err)
    }
}
```

This requires `NextID()` to be exported. In `internal/queue/queue.go`, rename `nextID()` to `NextID()` and update the call in `Create()`:

```go
// NextID returns the next available task ID.
func (q *Queue) NextID() int {
    // ... (same body as current nextID)
}

func (q *Queue) Create(description string, opts CreateOptions) (*Task, error) {
    id := q.NextID()
    // ... rest unchanged
}
```

Full updated `cmd/todo.go`:

```go
package cmd

import (
    "fmt"
    "os"

    "github.com/SeanoChang/cubit/internal/queue"
    "github.com/spf13/cobra"
)

var todoCmd = &cobra.Command{
    Use:   "todo <description>",
    Short: "Create a new task in the queue",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        ctx, _ := cmd.Flags().GetString("context")
        file, _ := cmd.Flags().GetString("file")
        mode, _ := cmd.Flags().GetString("mode")
        dependsOn, _ := cmd.Flags().GetIntSlice("depends-on")
        program, _ := cmd.Flags().GetString("program")
        goal, _ := cmd.Flags().GetString("goal")
        maxIter, _ := cmd.Flags().GetInt("max-iterations")
        branch, _ := cmd.Flags().GetString("branch")

        if file != "" {
            data, err := os.ReadFile(file)
            if err != nil {
                return fmt.Errorf("reading context file: %w", err)
            }
            if ctx != "" {
                ctx += "\n\n"
            }
            ctx += string(data)
        }

        if len(dependsOn) == 1 && dependsOn[0] == 0 {
            dependsOn = nil
        }

        if len(dependsOn) > 0 {
            nextID := q.NextID()
            if err := q.ValidateDependencies(nextID, dependsOn); err != nil {
                return fmt.Errorf("dependency validation: %w", err)
            }
        }

        task, err := q.Create(args[0], queue.CreateOptions{
            Context:       ctx,
            Mode:          mode,
            DependsOn:     dependsOn,
            Program:       program,
            Goal:          goal,
            MaxIterations: maxIter,
            Branch:        branch,
        })
        if err != nil {
            return err
        }
        fmt.Printf("created task %03d: %s\n", task.ID, task.Title)
        return nil
    },
}
```

### Step 2: Run full test suite

```bash
go test ./...
```

Expected: all PASS.

### Step 3: Build and smoke test

```bash
go build -o cubit .
./cubit todo "first task"
./cubit todo "bad task" --depends-on 99,1  # 99 doesn't exist but shouldn't cycle
```

Expected: tasks created without error (validation only blocks actual cycles).

### Step 4: Commit

```bash
git add cmd/todo.go internal/queue/queue.go
git commit -m "feat: validate cycle-free deps in cubit todo"
```

---

## Task 5: Per-task `model` field on `Task` + `--model` flag on `cubit todo`

**Files:**
- Modify: `internal/queue/task.go`
- Modify: `internal/queue/task_test.go`
- Modify: `cmd/todo.go`
- Modify: `cmd/root.go`

### Step 1: Write failing test for `model` field in `task_test.go`

Add to `internal/queue/task_test.go`:

```go
func TestParseTaskModelField(t *testing.T) {
    raw := `---
id: 1
status: pending
created: 2026-03-09T00:00:00Z
model: claude-sonnet-4-6
---

# my task
`
    task, err := ParseTask([]byte(raw))
    if err != nil {
        t.Fatalf("ParseTask: %v", err)
    }
    if task.Model != "claude-sonnet-4-6" {
        t.Errorf("Model = %q, want claude-sonnet-4-6", task.Model)
    }
}

func TestParseTaskModelFieldEmpty(t *testing.T) {
    raw := `---
id: 1
status: pending
created: 2026-03-09T00:00:00Z
---

# my task
`
    task, err := ParseTask([]byte(raw))
    if err != nil {
        t.Fatalf("ParseTask: %v", err)
    }
    if task.Model != "" {
        t.Errorf("Model = %q, want empty string", task.Model)
    }
}
```

### Step 2: Run to confirm FAIL

```bash
go test ./internal/queue/... -v -run TestParseTaskModel
```

Expected: FAIL with "task.Model undefined"

### Step 3: Add `Model` field to `Task` struct in `internal/queue/task.go`

```go
type Task struct {
    ID            int       `yaml:"id"`
    Status        string    `yaml:"status"`
    Created       time.Time `yaml:"created"`
    Mode          string    `yaml:"mode,omitempty"`
    Model         string    `yaml:"model,omitempty"`
    DependsOn     []int     `yaml:"depends_on,omitempty"`
    Program       string    `yaml:"program,omitempty"`
    Goal          string    `yaml:"goal,omitempty"`
    MaxIterations int       `yaml:"max_iterations,omitempty"`
    Branch        string    `yaml:"branch,omitempty"`
    Title         string    `yaml:"-"`
    Body          string    `yaml:"-"`
}
```

### Step 4: Add `Model` to `CreateOptions` in `internal/queue/queue.go`

```go
type CreateOptions struct {
    Context       string
    Mode          string
    Model         string
    DependsOn     []int
    Program       string
    Goal          string
    MaxIterations int
    Branch        string
}
```

And wire it through in `Create()`:

```go
task := &Task{
    // ... existing fields ...
    Model:         opts.Model,
    // ...
}
```

### Step 5: Run task tests to confirm PASS

```bash
go test ./internal/queue/... -v -run TestParseTaskModel
```

Expected: both PASS.

### Step 6: Add `--model` flag to `cubit todo` in `cmd/root.go`

In `init()`, after the existing `todoCmd.Flags()` registrations:

```go
todoCmd.Flags().String("model", "", "Claude model override for this task (default: config model)")
```

### Step 7: Read and use `--model` in `cmd/todo.go`

Add to the `RunE` function (alongside the other flag reads):

```go
model, _ := cmd.Flags().GetString("model")
```

And pass it to `CreateOptions`:

```go
task, err := q.Create(args[0], queue.CreateOptions{
    Context:       ctx,
    Mode:          mode,
    Model:         model,
    DependsOn:     dependsOn,
    Program:       program,
    Goal:          goal,
    MaxIterations: maxIter,
    Branch:        branch,
})
```

### Step 8: Run full test suite

```bash
go test ./...
```

Expected: all PASS.

### Step 9: Build and smoke test

```bash
go build -o cubit .
./cubit todo "use sonnet for this" --model claude-sonnet-4-6
./cubit queue
```

Expected: task created. `cubit queue` shows it. Task file has `model: claude-sonnet-4-6` in frontmatter.

### Step 10: Commit

```bash
git add internal/queue/task.go internal/queue/task_test.go internal/queue/queue.go cmd/todo.go cmd/root.go
git commit -m "feat: add per-task model field to Task + --model flag on cubit todo"
```

---

## Task 6: `cmd/graph.go` — print the DAG

**Files:**
- Create: `cmd/graph.go`
- Modify: `cmd/root.go`

### Step 1: Create `cmd/graph.go`

```go
package cmd

import (
    "fmt"
    "strings"

    "github.com/SeanoChang/cubit/internal/queue"
    "github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
    Use:   "graph",
    Short: "Print the task DAG with dependency status",
    RunE: func(cmd *cobra.Command, args []string) error {
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

        if len(nodes) == 0 {
            fmt.Println("no tasks")
            return nil
        }

        // Check for cycles and warn if found
        if err := queue.DetectCycle(nodes); err != nil {
            fmt.Printf("warning: %v\n\n", err)
        }

        for _, n := range nodes {
            fmt.Println(formatNode(n))
        }
        return nil
    },
}

// formatNode renders a single graph node as a display line.
func formatNode(n *queue.GraphNode) string {
    t := n.Task
    // Mode tag: [loop] or [once]
    modeTag := fmt.Sprintf("[%s]", t.Mode)

    // Truncate title to 30 chars for alignment
    title := t.Title
    if len(title) > 30 {
        title = title[:27] + "..."
    }

    left := fmt.Sprintf("  %03d %-6s %-30s", t.ID, modeTag, title)

    var right string
    switch n.Status {
    case queue.StatusDone:
        right = "→ DONE"
    case queue.StatusActive:
        right = "← ACTIVE"
    case queue.StatusReady:
        right = "✓ ready"
    case queue.StatusWaiting:
        deps := make([]string, len(t.DependsOn))
        for i, d := range t.DependsOn {
            deps[i] = fmt.Sprintf("%03d", d)
        }
        right = fmt.Sprintf("⏳ waiting on [%s]", strings.Join(deps, ", "))
    }

    return fmt.Sprintf("%-48s %s", left, right)
}
```

### Step 2: Register `graphCmd` in `cmd/root.go`

Add to the `init()` function, after `rootCmd.AddCommand(refreshCmd)`:

```go
// cubit graph
rootCmd.AddCommand(graphCmd)
```

### Step 3: Build and smoke test

```bash
go build -o cubit .

# Set up a test scenario
./cubit todo "arch sweep" --mode loop
./cubit todo "ablation" --depends-on 1
./cubit todo "write report" --depends-on 1
./cubit todo "publish" --depends-on 2,3
./cubit graph
```

Expected output (approximately):
```
  001 [loop]  arch sweep                       ✓ ready
  002 [once]  ablation                         ⏳ waiting on [001]
  003 [once]  write report                     ⏳ waiting on [001]
  004 [once]  publish                          ⏳ waiting on [002, 003]
```

Then pop and complete task 1, and run `cubit graph` again — 001 should show `→ DONE` and 002, 003 should show `✓ ready`.

### Step 4: Run full test suite

```bash
go test ./...
```

Expected: all PASS.

### Step 5: Commit

```bash
git add cmd/graph.go cmd/root.go
git commit -m "feat: add cubit graph command"
```

---

## Final verification

```bash
go build -o cubit .
go test ./...
./cubit graph --help
```

Expected: binary builds, all tests pass, `cubit graph` shows help text.
