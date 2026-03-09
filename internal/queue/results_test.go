package queue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadResults_Empty(t *testing.T) {
	dir := t.TempDir()
	content := ReadResults(dir)
	if content != "" {
		t.Errorf("expected empty, got %q", content)
	}
}

func TestReadResults_Exists(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)
	os.WriteFile(filepath.Join(memDir, "results.tsv"), []byte("commit\tval_bpb\tstatus\na1b2\t0.98\tkept\n"), 0o644)

	content := ReadResults(dir)
	if !strings.Contains(content, "a1b2") {
		t.Errorf("expected results content, got %q", content)
	}
}

func TestAppendResult(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)

	err := AppendResult(dir, "a1b2\t0.98\tkept\tswitch to GQA")
	if err != nil {
		t.Fatalf("AppendResult: %v", err)
	}

	content := ReadResults(dir)
	if !strings.Contains(content, "a1b2") {
		t.Errorf("expected appended content, got %q", content)
	}

	err = AppendResult(dir, "c3d4\t0.99\tdiscarded\ttry SwiGLU")
	if err != nil {
		t.Fatalf("AppendResult second: %v", err)
	}

	content = ReadResults(dir)
	if !strings.Contains(content, "c3d4") {
		t.Errorf("expected second row, got %q", content)
	}
}
