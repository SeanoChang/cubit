package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRoot(t *testing.T) {
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".ark")
	got := DefaultRoot()
	if got != want {
		t.Errorf("DefaultRoot() = %q, want %q", got, want)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := Default("noah")
	if cfg.Agent != "noah" {
		t.Errorf("Agent = %q, want %q", cfg.Agent, "noah")
	}
	home, _ := os.UserHomeDir()
	wantRoot := filepath.Join(home, ".ark")
	if cfg.Root != wantRoot {
		t.Errorf("Root = %q, want %q", cfg.Root, wantRoot)
	}
}

func TestAgentDir(t *testing.T) {
	cfg := &Config{Agent: "noah", Root: "/tmp/ark"}
	want := "/tmp/ark/noah"
	got := cfg.AgentDir()
	if got != want {
		t.Errorf("AgentDir() = %q, want %q", got, want)
	}
}
