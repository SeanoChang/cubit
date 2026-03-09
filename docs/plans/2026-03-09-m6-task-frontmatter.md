# M6: Task Frontmatter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `mode`, `depends_on`, `program`, `goal`, `max_iterations`, `branch` fields to the Task struct and wire them through `cubit todo` flags.

**Architecture:** Extend `Task` with 6 new YAML frontmatter fields; introduce a `CreateOptions` struct in `queue.go` to pass optional fields through `Queue.Create`; register new flags in `root.go`. Existing tasks parse cleanly — `mode` defaults to `"once"`, all other new fields default to zero values.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, Cobra

---

### Task 1: Extend Task struct + ParseTask defaults

**Files:**
- Modify: `internal/queue/task.go`
- Modify: `internal/queue/task_test.go`

**Step 1: Write failing tests**

Add to `internal/queue/task_test.go`:

```go
func TestParseTaskWithNewFields(t *testing.T) {
	raw := `---
id: 1
status: pending
created: 2026-03-09T00:00:00Z
mode: loop
depends_on: [2, 3]
program: sweep.md
goal: "val_bpb < 0.95"
max_iterations: 100
branch: noah/sweep-arch
---

# Architecture sweep
`
	task, err := ParseTask([]byte(raw))
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	if task.Mode != "loop" {
		t.Errorf("Mode = %q, want loop", task.Mode)
	}
	if len(task.DependsOn) != 2 || task.DependsOn[0] != 2 || task.DependsOn[1] != 3 {
		t.Errorf("DependsOn = %v, want [2 3]", task.DependsOn)
	}
	if task.Program != "sweep.md" {
		t.Errorf("Program = %q, want sweep.md", task.Program)
	}
	if task.Goal != "val_bpb < 0.95" {
		t.Errorf("Goal = %q, want val_bpb < 0.95", task.Goal)
	}
	if task.MaxIterations != 100 {
		t.Errorf("MaxIterations = %d, want 100", task.MaxIterations)
	}
	if task.Branch != "noah/sweep-arch" {
		t.Errorf("Branch = %q, want noah/sweep-arch", task.Branch)
	}
}

func TestParseTaskDefaultsMode(t *testing.T) {
	raw := `---
id: 1
status: pending
created: 2026-03-09T00:00:00Z
---

# simple task
`
	task, err := ParseTask([]byte(raw))
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	if task.Mode != "once" {
		t.Errorf("Mode = %q, want once (default)", task.Mode)
	}
	if task.DependsOn != nil {
		t.Errorf("DependsOn = %v, want nil", task.DependsOn)
	}
}
```

**Step 2: Run tests to verify they fail**

```
go test ./internal/queue/ -run "TestParseTaskWithNewFields|TestParseTaskDefaultsMode" -v
```

Expected: FAIL — fields don't exist on Task struct yet.

**Step 3: Add fields to Task struct and default Mode in ParseTask**

In `internal/queue/task.go`, update `Task`:

```go
type Task struct {
	ID            int       `yaml:"id"`
	Status        string    `yaml:"status"`
	Created       time.Time `yaml:"created"`
	Mode          string    `yaml:"mode,omitempty"`
	DependsOn     []int     `yaml:"depends_on,omitempty"`
	Program       string    `yaml:"program,omitempty"`
	Goal          string    `yaml:"goal,omitempty"`
	MaxIterations int       `yaml:"max_iterations,omitempty"`
	Branch        string    `yaml:"branch,omitempty"`
	Title         string    `yaml:"-"` // extracted from body
	Body          string    `yaml:"-"` // markdown body after frontmatter
}
```

In `ParseTask`, after `yaml.Unmarshal` succeeds, add:

```go
if task.Mode == "" {
    task.Mode = "once"
}
```

**Step 4: Run tests**

```
go test ./internal/queue/ -v
```

Expected: all PASS (new tests + existing round-trip test).

**Step 5: Commit**

```bash
git add internal/queue/task.go internal/queue/task_test.go
git commit -m "feat(queue): add mode/depends_on/program/goal/max_iterations/branch to Task"
```

---

### Task 2: Update Queue.Create to accept CreateOptions

**Files:**
- Modify: `internal/queue/queue.go`
- Modify: `internal/queue/queue_test.go`
- Modify: `cmd/todo.go` (fix the now-broken Create call)

**Step 1: Write failing tests**

Add to `internal/queue/queue_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

```
go test ./internal/queue/ -run "TestCreateWith|TestCreateDefaults" -v
```

Expected: FAIL — `CreateOptions` undefined, `Create` signature mismatch.

**Step 3: Add CreateOptions and update Create in queue.go**

Add `CreateOptions` struct before `Create`:

```go
// CreateOptions holds optional metadata for a new task.
type CreateOptions struct {
	Context       string
	Mode          string
	DependsOn     []int
	Program       string
	Goal          string
	MaxIterations int
	Branch        string
}
```

Update `Create` signature and body:

```go
func (q *Queue) Create(description string, opts CreateOptions) (*Task, error) {
	id := q.nextID()

	body := fmt.Sprintf("# %s", description)
	if opts.Context != "" {
		body += "\n\n" + opts.Context
	}

	mode := opts.Mode
	if mode == "" {
		mode = "once"
	}

	task := &Task{
		ID:            id,
		Status:        "pending",
		Created:       time.Now().UTC().Truncate(time.Second),
		Mode:          mode,
		DependsOn:     opts.DependsOn,
		Program:       opts.Program,
		Goal:          opts.Goal,
		MaxIterations: opts.MaxIterations,
		Branch:        opts.Branch,
		Title:         description,
		Body:          strings.TrimSpace(body),
	}

	slug := Slugify(description)
	filename := fmt.Sprintf("%03d-%s.md", id, slug)
	path := filepath.Join(q.queueDir, filename)

	if err := os.WriteFile(path, task.Serialize(), 0o644); err != nil {
		return nil, fmt.Errorf("writing task file: %w", err)
	}
	return task, nil
}
```

**Step 4: Fix the broken caller in cmd/todo.go**

Change:
```go
task, err := q.Create(args[0], ctx)
```
To:
```go
task, err := q.Create(args[0], queue.CreateOptions{Context: ctx})
```

**Step 5: Run all tests**

```
go test ./... -v
```

Expected: all PASS.

**Step 6: Commit**

```bash
git add internal/queue/queue.go internal/queue/queue_test.go cmd/todo.go
git commit -m "feat(queue): introduce CreateOptions struct, update Create signature"
```

---

### Task 3: Wire new flags to cubit todo

**Files:**
- Modify: `cmd/todo.go`
- Modify: `cmd/root.go`

**Step 1: Update todo.go to read new flags**

Replace the body of `todoCmd` with:

```go
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

		// Cobra parses --depends-on as IntSlice but returns [0] for unset.
		// Treat a single 0 as "not provided".
		if len(dependsOn) == 1 && dependsOn[0] == 0 {
			dependsOn = nil
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

**Step 2: Register new flags in root.go**

Replace the `cubit todo` block in `init()`:

```go
// cubit todo "description" [--context "..."] [-f file.md]
//   [--mode once|loop] [--depends-on 1,2] [--program file.md]
//   [--goal "expr"] [--max-iterations N] [--branch name]
todoCmd.Flags().StringP("context", "c", "", "Inline context to append to task body")
todoCmd.Flags().StringP("file", "f", "", "Read context from file")
todoCmd.Flags().String("mode", "once", "Execution mode: once or loop")
todoCmd.Flags().IntSlice("depends-on", nil, "Comma-separated task IDs this task depends on")
todoCmd.Flags().String("program", "", "Program file re-injected each loop iteration")
todoCmd.Flags().String("goal", "", "Exit condition for loop mode (agent evaluates)")
todoCmd.Flags().Int("max-iterations", 0, "Maximum loop iterations (0 = unlimited)")
todoCmd.Flags().String("branch", "", "Git branch for this task (convention, not enforced)")
rootCmd.AddCommand(todoCmd)
```

**Step 3: Build and smoke test**

```bash
go build -o cubit .
./cubit todo "arch sweep" --mode loop --depends-on 1,2 --program sweep.md \
  --goal "val_bpb < 0.95" --max-iterations 100 --branch noah/sweep-arch
```

Expected output: `created task 001: arch sweep`

Inspect the written file (path will be something like `~/.ark/cubit/noah/queue/001-arch-sweep.md`):

```
cat ~/.ark/cubit/noah/queue/001-arch-sweep.md
```

Expected: frontmatter contains all 6 fields correctly.

Also verify defaults — no new flags:

```bash
./cubit todo "simple task"
cat ~/.ark/cubit/noah/queue/002-simple-task.md
```

Expected: `mode: once`, no other new fields in frontmatter.

**Step 4: Run all tests**

```
go test ./... -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add cmd/todo.go cmd/root.go
git commit -m "feat(cmd): wire mode/depends-on/program/goal/max-iterations/branch flags to cubit todo"
```

---

## M6 Done

Exit criteria met:
- `cubit todo` accepts all 6 new flags
- Task files written with full new frontmatter
- Existing tasks (no new flags) default to `mode: once`, no deps
- All tests pass
