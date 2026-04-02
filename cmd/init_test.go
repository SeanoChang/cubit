package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInitKeelWritesPending(t *testing.T) {
	root := t.TempDir()
	agent := "testbot"
	agentDir := filepath.Join(root, "agents-home", agent)

	// Write .init-pending the same way the --keel flag does
	pending := InitPending{
		Agent:          agent,
		RequestedAt:    time.Now().UTC().Format(time.RFC3339),
		ImportIdentity: false,
	}
	if err := writeInitPending(agentDir, pending); err == nil {
		// agentDir doesn't exist yet — scaffold first
		t.Log("expected error before scaffold, that's fine")
	}

	// Scaffold first
	os.MkdirAll(agentDir, 0755)
	if err := writeInitPending(agentDir, pending); err != nil {
		t.Fatalf("writeInitPending() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(agentDir, ".init-pending"))
	if err != nil {
		t.Fatalf("read .init-pending: %v", err)
	}

	var got InitPending
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Agent != agent {
		t.Errorf("Agent = %q, want %q", got.Agent, agent)
	}
	if got.ImportIdentity != false {
		t.Error("ImportIdentity should be false")
	}
}
