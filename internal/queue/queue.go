// Package queue manages the task lifecycle for a cubit agent.
package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Queue manages task files in an agent's queue/ directory.
type Queue struct {
	queueDir   string // path to queue/
	scratchDir string // path to scratch/
	logPath    string // path to memory/log.md
}

var instance *Queue

// ResetForTest clears the singleton. Test-only.
func ResetForTest() {
	instance = nil
}

// GetQueue returns the singleton Queue, initializing it on first call.
func GetQueue(agentDir string) *Queue {
	if instance == nil {
		qDir := filepath.Join(agentDir, "queue")
		instance = &Queue{
			queueDir:   qDir,
			scratchDir: filepath.Join(agentDir, "scratch"),
			logPath:    filepath.Join(agentDir, "memory", "log.md"),
		}
		if err := os.MkdirAll(filepath.Join(qDir, ".doing"), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "warning: creating .doing/ dir: %v\n", err)
		}
	}
	return instance
}

// CreateOptions holds optional metadata for a new task.
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

// Create adds a new task to the queue. Returns the created task.
func (q *Queue) Create(description string, opts CreateOptions) (*Task, error) {
	id := q.NextID()

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
		Model:         opts.Model,
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

// List returns all pending tasks sorted by ID.
func (q *Queue) List() ([]*Task, error) {
	entries, err := filepath.Glob(filepath.Join(q.queueDir, "*.md"))
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
			continue // skip malformed files
		}
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})
	return tasks, nil
}

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

// Pop moves the lowest-ID pending task to .doing/ directory. Returns the task.
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

// Active returns all currently active tasks in .doing/ directory.
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

// findActiveByID finds an active task by its ID in the .doing/ directory.
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

// CompleteByID finishes a specific active task by ID and appends to log.md.
func (q *Queue) CompleteByID(id int, summary string) error {
	path, task, err := q.findActiveByID(id)
	if err != nil {
		return err
	}

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

// RequeueByID returns a specific active task by ID to the queue as pending.
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

// Complete finishes the active task and appends to log.md.
// Errors if multiple tasks are active — use CompleteByID instead.
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

// Requeue returns the active task to the queue as pending.
// Errors if multiple tasks are active — use RequeueByID instead.
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


// Log appends a free-form observation to log.md.
func (q *Queue) Log(note string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	entry := fmt.Sprintf("\n## %s — observation\n%s\n", now, note)
	return q.appendLog(entry)
}

func (q *Queue) appendLog(entry string) error {
	f, err := os.OpenFile(q.logPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("opening log: %w", err)
	}
	if _, err = f.WriteString(entry); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing log: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("closing log: %w", err)
	}
	return nil
}

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
	activeTasks, err := q.Active()
	if err != nil {
		return err
	}

	// Check all dep IDs exist in pending/active/done
	knownIDs := make(map[int]bool)
	for _, t := range pending {
		knownIDs[t.ID] = true
	}
	for _, t := range done {
		knownIDs[t.ID] = true
	}
	for _, t := range activeTasks {
		knownIDs[t.ID] = true
	}
	for _, dep := range deps {
		if !knownIDs[dep] {
			return fmt.Errorf("dependency %03d does not exist", dep)
		}
	}

	// Add hypothetical new task and check for cycles
	newTask := &Task{ID: newID, DependsOn: deps}
	pending = append(pending, newTask)
	nodes := BuildGraph(pending, activeTasks, done)
	return DetectCycle(nodes)
}

// NextID scans queue/ (including .doing/ and done/) for the highest existing ID and returns ID+1.
func (q *Queue) NextID() int {
	maxID := 0

	// Check queued files
	entries, _ := filepath.Glob(filepath.Join(q.queueDir, "*.md"))
	for _, path := range entries {
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

	// Check done/
	doneEntries, _ := filepath.Glob(filepath.Join(q.queueDir, "done", "*.md"))
	for _, path := range doneEntries {
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

	return maxID + 1
}
