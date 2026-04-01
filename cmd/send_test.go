package cmd

import (
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

