package brief

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile creates a file at dir/relPath with the given content,
// creating parent directories as needed.
func writeTestFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	abs := filepath.Join(dir, relPath)
	os.MkdirAll(filepath.Dir(abs), 0o755)
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile(%s): %v", relPath, err)
	}
}

func setupBriefTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "identity"), 0o755)
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.MkdirAll(filepath.Join(dir, "queue", ".doing"), 0o755)
	os.MkdirAll(filepath.Join(dir, "scratch"), 0o755)
	os.WriteFile(filepath.Join(dir, "identity", "FLUCTLIGHT.md"), []byte("I am an agent"), 0o644)
	return dir
}

func TestBuildWithUpstream_InjectsOutputPaths(t *testing.T) {
	dir := setupBriefTestDir(t)

	os.WriteFile(filepath.Join(dir, "scratch", "001-output.md"), []byte("result 1"), 0o644)
	os.WriteFile(filepath.Join(dir, "scratch", "002-output.md"), []byte("result 2"), 0o644)

	result := BuildWithUpstream(dir, []int{1, 2})

	if !strings.Contains(result, "## Upstream Results") {
		t.Error("missing Upstream Results section")
	}
	if !strings.Contains(result, "scratch/001-output.md") {
		t.Error("missing output path for task 1")
	}
	if !strings.Contains(result, "scratch/002-output.md") {
		t.Error("missing output path for task 2")
	}
}

func TestBuildWithUpstream_SkipsMissingOutputs(t *testing.T) {
	dir := setupBriefTestDir(t)

	os.WriteFile(filepath.Join(dir, "scratch", "001-output.md"), []byte("result 1"), 0o644)

	result := BuildWithUpstream(dir, []int{1, 2})

	if !strings.Contains(result, "001-output.md") {
		t.Error("should include existing output")
	}
	if strings.Contains(result, "002-output.md") {
		t.Error("should not include missing output")
	}
}

func TestBuildWithUpstream_NoUpstream(t *testing.T) {
	dir := setupBriefTestDir(t)

	result := BuildWithUpstream(dir, nil)

	if strings.Contains(result, "Upstream Results") {
		t.Error("should not have Upstream Results with no upstream IDs")
	}
}
