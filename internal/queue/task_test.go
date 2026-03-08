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
