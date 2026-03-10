package brief

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractNarkID(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"found", "some output\nnark: abc123def\nmore", "abc123def"},
		{"not found", "no id here", ""},
		{"empty", "", ""},
		{"inline nark", "nark: ff00aa", "ff00aa"},
		{"with extra spaces", "nark:   deadbeef", "deadbeef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractNarkID(tt.input)
			if got != tt.expect {
				t.Errorf("ExtractNarkID(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestRunPostDrainLifecycle_WithNarkID(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.MkdirAll(filepath.Join(dir, "scratch"), 0o755)

	// Seed files
	os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte("# Goal\nDo something"), 0o644)
	os.WriteFile(filepath.Join(dir, "memory", "brief.md"), []byte("old brief content"), 0o644)
	os.WriteFile(filepath.Join(dir, "memory", "MEMORY.md"), []byte("# Memory\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "scratch", "001-output.md"), []byte("task 1 output"), 0o644)
	os.WriteFile(filepath.Join(dir, "scratch", "002-observations.md"), []byte("observations"), 0o644)
	os.WriteFile(filepath.Join(dir, "scratch", "iter-001.txt"), []byte("3"), 0o644)

	err := RunPostDrainLifecycle(dir, "summary\nnark: abc123\ndone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GOALS.md should be empty
	goals, _ := os.ReadFile(filepath.Join(dir, "GOALS.md"))
	if len(goals) != 0 {
		t.Errorf("GOALS.md should be empty, got %q", goals)
	}

	// brief.md should be slim
	brief, _ := os.ReadFile(filepath.Join(dir, "memory", "brief.md"))
	if !strings.Contains(string(brief), "nark: abc123") {
		t.Errorf("brief.md should contain nark ID, got %q", brief)
	}

	// log.md should have entry
	log, _ := os.ReadFile(filepath.Join(dir, "memory", "log.md"))
	if !strings.Contains(string(log), "goal cycle completed") {
		t.Errorf("log.md should have cycle entry, got %q", log)
	}
	if !strings.Contains(string(log), "abc123") {
		t.Errorf("log.md should have nark ID, got %q", log)
	}

	// MEMORY.md should have pointer
	mem, _ := os.ReadFile(filepath.Join(dir, "memory", "MEMORY.md"))
	if !strings.Contains(string(mem), "Archived to nark: abc123") {
		t.Errorf("MEMORY.md should have archive pointer, got %q", mem)
	}

	// scratch/ should be cleaned
	mdFiles, _ := filepath.Glob(filepath.Join(dir, "scratch", "*.md"))
	if len(mdFiles) != 0 {
		t.Errorf("scratch/*.md should be cleaned, found %v", mdFiles)
	}
	txtFiles, _ := filepath.Glob(filepath.Join(dir, "scratch", "*.txt"))
	if len(txtFiles) != 0 {
		t.Errorf("scratch/*.txt should be cleaned, found %v", txtFiles)
	}
}

func TestRunPostDrainLifecycle_WithoutNarkID(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.MkdirAll(filepath.Join(dir, "scratch"), 0o755)

	os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte("# Goal"), 0o644)
	os.WriteFile(filepath.Join(dir, "memory", "brief.md"), []byte("old brief"), 0o644)
	os.WriteFile(filepath.Join(dir, "memory", "MEMORY.md"), []byte("# Memory\n"), 0o644)

	err := RunPostDrainLifecycle(dir, "output without nark id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GOALS.md should be empty
	goals, _ := os.ReadFile(filepath.Join(dir, "GOALS.md"))
	if len(goals) != 0 {
		t.Errorf("GOALS.md should be empty, got %q", goals)
	}

	// brief.md should not reference nark
	brief, _ := os.ReadFile(filepath.Join(dir, "memory", "brief.md"))
	if strings.Contains(string(brief), "nark") {
		t.Errorf("brief.md should not contain nark, got %q", brief)
	}
	if !strings.Contains(string(brief), "Previous cycle completed") {
		t.Errorf("brief.md should have cycle message, got %q", brief)
	}

	// log.md should note missing nark
	log, _ := os.ReadFile(filepath.Join(dir, "memory", "log.md"))
	if !strings.Contains(string(log), "no nark ID found") {
		t.Errorf("log.md should note missing nark, got %q", log)
	}

	// MEMORY.md should NOT be modified (no pointer without nark ID)
	mem, _ := os.ReadFile(filepath.Join(dir, "memory", "MEMORY.md"))
	if strings.Contains(string(mem), "Archived") {
		t.Errorf("MEMORY.md should not have archive pointer without nark ID, got %q", mem)
	}
}
