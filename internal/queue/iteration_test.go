package queue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetIteration_NoFile(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	n := GetIteration(scratchDir, 1)
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestIncrementIteration(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	n := IncrementIteration(scratchDir, 1)
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}

	n = IncrementIteration(scratchDir, 1)
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}

	got := GetIteration(scratchDir, 1)
	if got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestClearIteration(t *testing.T) {
	dir := t.TempDir()
	scratchDir := filepath.Join(dir, "scratch")
	os.MkdirAll(scratchDir, 0o755)

	IncrementIteration(scratchDir, 1)
	IncrementIteration(scratchDir, 1)
	ClearIteration(scratchDir, 1)

	got := GetIteration(scratchDir, 1)
	if got != 0 {
		t.Errorf("expected 0 after clear, got %d", got)
	}
}
