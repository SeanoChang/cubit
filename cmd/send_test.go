package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	input := `---
from: alice
to: noah
subject: Found a regression in auth module
category: important
type: notification
---

Body of the message here.
Second line.
`

	fm, body, err := parseFrontmatter(input)
	if err != nil {
		t.Fatalf("parseFrontmatter error: %v", err)
	}

	if fm["from"] != "alice" {
		t.Errorf("from = %q, want alice", fm["from"])
	}
	if fm["to"] != "noah" {
		t.Errorf("to = %q, want noah", fm["to"])
	}
	if fm["subject"] != "Found a regression in auth module" {
		t.Errorf("subject = %q", fm["subject"])
	}
	if fm["category"] != "important" {
		t.Errorf("category = %q, want important", fm["category"])
	}

	if body != "Body of the message here.\nSecond line." {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatterNoFrontmatter(t *testing.T) {
	_, _, err := parseFrontmatter("Just plain text")
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestParseFrontmatterUnterminated(t *testing.T) {
	_, _, err := parseFrontmatter("---\nfrom: alice\nto: noah\n")
	if err == nil {
		t.Error("expected error for unterminated frontmatter")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Found a regression in auth module", "found-a-regression-in-auth-module"},
		{"Hello, World!", "hello-world"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"UPPERCASE", "uppercase"},
		{"special@#$chars", "specialchars"},
		{"", ""},
		{"a-b-c", "a-b-c"},
	}

	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildMessage(t *testing.T) {
	fm := map[string]string{
		"from":    "alice",
		"to":      "noah",
		"subject": "Test message",
	}
	body := "Hello!\n"

	result := buildMessage(fm, body)

	// Should have frontmatter delimiters
	if result[:4] != "---\n" {
		t.Error("missing opening ---")
	}
	if !strings.Contains(result, "from: alice") {
		t.Error("missing from field")
	}
	if !strings.Contains(result, "to: noah") {
		t.Error("missing to field")
	}
	if !strings.Contains(result, "subject: Test message") {
		t.Error("missing subject field")
	}
	if !strings.Contains(result, "Hello!") {
		t.Error("missing body")
	}
}

func TestParseFrontmatterWithNewFields(t *testing.T) {
	input := `---
from: alice
to: noah
subject: Delegation task
category: important
type: handoff
delegation_id: del-abc123
attempt: 1
reply_to: alice
---

Please handle this task.
`

	fm, body, err := parseFrontmatter(input)
	if err != nil {
		t.Fatalf("parseFrontmatter error: %v", err)
	}

	checks := map[string]string{
		"from":          "alice",
		"to":            "noah",
		"subject":       "Delegation task",
		"category":      "important",
		"type":          "handoff",
		"delegation_id": "del-abc123",
		"attempt":       "1",
		"reply_to":      "alice",
	}

	for key, want := range checks {
		if got := fm[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	if !strings.Contains(body, "Please handle this task.") {
		t.Errorf("body = %q, expected to contain task text", body)
	}
}

func TestBuildMessageWithNewFields(t *testing.T) {
	fm := map[string]string{
		"from":          "alice",
		"to":            "noah",
		"subject":       "Delegation task",
		"type":          "handoff",
		"delegation_id": "del-abc123",
		"attempt":       "1",
		"reply_to":      "alice",
		"timestamp":     "2026-01-01T00:00:00Z",
	}
	body := "Task body.\n"

	result := buildMessage(fm, body)

	// All new fields should appear in the output
	for _, field := range []string{"delegation_id: del-abc123", "attempt: 1", "reply_to: alice", "type: handoff"} {
		if !strings.Contains(result, field) {
			t.Errorf("missing field %q in output:\n%s", field, result)
		}
	}

	// Verify ordering: delegation_id should come after type in the ordered output
	typeIdx := strings.Index(result, "type: handoff")
	delIdx := strings.Index(result, "delegation_id: del-abc123")
	attemptIdx := strings.Index(result, "attempt: 1")
	replyIdx := strings.Index(result, "reply_to: alice")

	if typeIdx > delIdx {
		t.Error("type should appear before delegation_id")
	}
	if delIdx > attemptIdx {
		t.Error("delegation_id should appear before attempt")
	}
	if attemptIdx > replyIdx {
		t.Error("attempt should appear before reply_to")
	}
}

func TestCopyDir(t *testing.T) {
	// Create source directory structure
	srcDir := t.TempDir()

	// Create mail.md
	mailContent := `---
from: alice
to: noah
subject: Test
---

Body here.
`
	if err := os.WriteFile(filepath.Join(srcDir, "mail.md"), []byte(mailContent), 0o644); err != nil {
		t.Fatalf("writing mail.md: %v", err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(srcDir, "attachments")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("creating subdirectory: %v", err)
	}
	attachContent := "attachment data"
	if err := os.WriteFile(filepath.Join(subDir, "file.txt"), []byte(attachContent), 0o644); err != nil {
		t.Fatalf("writing attachment: %v", err)
	}

	// Create another file at the root level
	if err := os.WriteFile(filepath.Join(srcDir, "context.json"), []byte(`{"key":"value"}`), 0o644); err != nil {
		t.Fatalf("writing context.json: %v", err)
	}

	// Copy to a new destination
	dstDir := filepath.Join(t.TempDir(), "copied")
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	// Verify mail.md was copied with correct content
	data, err := os.ReadFile(filepath.Join(dstDir, "mail.md"))
	if err != nil {
		t.Fatalf("reading copied mail.md: %v", err)
	}
	if string(data) != mailContent {
		t.Errorf("mail.md content mismatch:\ngot:  %q\nwant: %q", string(data), mailContent)
	}

	// Verify subdirectory and file were copied
	data, err = os.ReadFile(filepath.Join(dstDir, "attachments", "file.txt"))
	if err != nil {
		t.Fatalf("reading copied attachment: %v", err)
	}
	if string(data) != attachContent {
		t.Errorf("attachment content mismatch: got %q, want %q", string(data), attachContent)
	}

	// Verify context.json was copied
	data, err = os.ReadFile(filepath.Join(dstDir, "context.json"))
	if err != nil {
		t.Fatalf("reading copied context.json: %v", err)
	}
	if string(data) != `{"key":"value"}` {
		t.Errorf("context.json content mismatch: got %q", string(data))
	}

	// Verify the destination directory exists
	info, err := os.Stat(dstDir)
	if err != nil {
		t.Fatalf("stat dstDir: %v", err)
	}
	if !info.IsDir() {
		t.Error("destination should be a directory")
	}

	// Verify the attachments subdirectory exists
	info, err = os.Stat(filepath.Join(dstDir, "attachments"))
	if err != nil {
		t.Fatalf("stat attachments dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("attachments should be a directory")
	}
}

