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
	queueDir string // path to queue/
	logPath  string // path to memory/log.md
}

var instance *Queue

// GetQueue returns the singleton Queue, initializing it on first call.
func GetQueue(agentDir string) *Queue {
	if instance == nil {
		instance = &Queue{
			queueDir: filepath.Join(agentDir, "queue"),
			logPath:  filepath.Join(agentDir, "memory", "log.md"),
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

// Pop moves the lowest-ID pending task to .doing. Returns the task.
func (q *Queue) Pop() (*Task, error) {
	doingPath := filepath.Join(q.queueDir, ".doing")
	if _, err := os.Stat(doingPath); err == nil {
		return nil, fmt.Errorf("a task is already active (see .doing)")
	}

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
		if err := os.WriteFile(doingPath, task.Serialize(), 0o644); err != nil {
			return nil, fmt.Errorf("writing .doing: %w", err)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("removing original task file: %w", err)
		}
		return task, nil
	}

	return nil, fmt.Errorf("queue is empty")
}

// Active returns the currently active task, or nil if none.
func (q *Queue) Active() (*Task, error) {
	doingPath := filepath.Join(q.queueDir, ".doing")
	data, err := os.ReadFile(doingPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseTask(data)
}

// Complete finishes the active task and appends to log.md.
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

	return os.Remove(filepath.Join(q.queueDir, ".doing"))
}

// Requeue returns the active task to the queue as pending.
func (q *Queue) Requeue() error {
	task, err := q.Active()
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("no active task")
	}

	task.Status = "pending"
	slug := Slugify(task.Title)
	filename := fmt.Sprintf("%03d-%s.md", task.ID, slug)
	path := filepath.Join(q.queueDir, filename)

	if err := os.WriteFile(path, task.Serialize(), 0o644); err != nil {
		return err
	}
	return os.Remove(filepath.Join(q.queueDir, ".doing"))
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
	active, err := q.Active()
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
	if active != nil {
		knownIDs[active.ID] = true
	}
	for _, dep := range deps {
		if !knownIDs[dep] {
			return fmt.Errorf("dependency %03d does not exist", dep)
		}
	}

	// Add hypothetical new task and check for cycles
	newTask := &Task{ID: newID, DependsOn: deps}
	pending = append(pending, newTask)
	nodes := BuildGraph(pending, active, done)
	return DetectCycle(nodes)
}

// NextID scans queue/ (including .doing and done/) for the highest existing ID and returns ID+1.
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

	// Check .doing
	doingPath := filepath.Join(q.queueDir, ".doing")
	if data, err := os.ReadFile(doingPath); err == nil {
		if t, err := ParseTask(data); err == nil && t.ID > maxID {
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
