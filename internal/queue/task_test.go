package queue

import (
	"testing"
	"time"
)

func TestParseTask(t *testing.T) {
	raw := `---
id: 3
status: pending
created: 2026-03-08T14:00:00Z
---

# implement FTS5 insert

Some context here.
`
	task, err := ParseTask([]byte(raw))
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	if task.ID != 3 {
		t.Errorf("ID = %d, want 3", task.ID)
	}
	if task.Status != "pending" {
		t.Errorf("Status = %q, want pending", task.Status)
	}
	if task.Created.Year() != 2026 {
		t.Errorf("Created year = %d, want 2026", task.Created.Year())
	}
	if task.Title != "implement FTS5 insert" {
		t.Errorf("Title = %q, want %q", task.Title, "implement FTS5 insert")
	}
	if task.Body == "" {
		t.Error("Body is empty")
	}
}

func TestSerializeTask(t *testing.T) {
	task := &Task{
		ID:      1,
		Status:  "pending",
		Created: time.Date(2026, 3, 8, 14, 0, 0, 0, time.UTC),
		Title:   "test task",
		Body:    "# test task\n\nSome body.",
	}
	data := task.Serialize()
	// Round-trip
	parsed, err := ParseTask(data)
	if err != nil {
		t.Fatalf("round-trip ParseTask: %v", err)
	}
	if parsed.ID != task.ID || parsed.Status != task.Status || parsed.Title != task.Title {
		t.Errorf("round-trip mismatch: got %+v", parsed)
	}
}

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
		t.Errorf("Model = %q, want empty", task.Model)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Implement FTS5 Insert Logic", "implement-fts5-insert-logic"},
		{"hello world foo bar baz qux", "hello-world-foo-bar-baz"},
		{"already-slugged", "already-slugged"},
		{"  spaces & symbols!! ", "spaces-symbols"},
	}
	for _, tt := range tests {
		got := Slugify(tt.in)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
