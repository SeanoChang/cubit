package queue

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadResults returns the contents of memory/results.tsv, or "" if missing.
func ReadResults(agentDir string) string {
	data, err := os.ReadFile(filepath.Join(agentDir, "memory", "results.tsv"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// AppendResult appends a single TSV row to memory/results.tsv.
func AppendResult(agentDir string, row string) error {
	path := filepath.Join(agentDir, "memory", "results.tsv")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if !strings.HasSuffix(row, "\n") {
		row += "\n"
	}
	_, err = f.WriteString(row)
	return err
}
