package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	root := t.TempDir()
	agent := "testbot"

	created, err := Init(root, agent, false)
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if !created {
		t.Fatal("Init() returned created=false, want true")
	}

	agentDir := filepath.Join(root, "agents-home", agent)

	// Check directories exist
	dirs := []string{
		"scratch",
		"projects",
		"memory",
		".claude",
		".claude/agents",
	}
	for _, d := range dirs {
		path := filepath.Join(agentDir, d)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("directory %s not found: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}

	// Check files exist
	files := []string{
		"FLUCTLIGHT.md",
		"PROGRAM.md",
		"GOALS.md",
		"MEMORY.md",
		"log.md",
		".claude/settings.json",
		".claude/agents/testbot.md",
	}
	for _, f := range files {
		path := filepath.Join(agentDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("file %s not found: %v", f, err)
		}
	}

	// Check .git/ does NOT exist at workspace root (no git init)
	gitDir := filepath.Join(agentDir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		t.Error(".git/ should not exist at workspace root")
	}

	// Check agent.md contains agent name
	agentMD, _ := os.ReadFile(filepath.Join(agentDir, ".claude", "agents", "testbot.md"))
	if !strings.Contains(string(agentMD), "name: testbot") {
		t.Error("agent.md does not contain agent name")
	}

	// Check settings.json has allow list
	settings, _ := os.ReadFile(filepath.Join(agentDir, ".claude", "settings.json"))
	if !strings.Contains(string(settings), "allow") {
		t.Error("settings.json missing allow list")
	}
}

func TestInitAlreadyExists(t *testing.T) {
	root := t.TempDir()
	agent := "testbot"

	Init(root, agent, false)

	created, err := Init(root, agent, false)
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if created {
		t.Fatal("Init() returned created=true for existing agent, want false")
	}
}

func TestInitForce(t *testing.T) {
	root := t.TempDir()
	agent := "testbot"

	Init(root, agent, false)

	// Corrupt agent.md
	agentMDPath := filepath.Join(root, "agents-home", agent, ".claude", "agents", "testbot.md")
	os.WriteFile(agentMDPath, []byte("corrupted"), 0o644)

	// Force re-init
	created, err := Init(root, agent, true)
	if err != nil {
		t.Fatalf("Init(force=true) error: %v", err)
	}
	if !created {
		t.Fatal("Init(force=true) returned created=false, want true")
	}

	// Verify agent.md was recreated
	data, _ := os.ReadFile(agentMDPath)
	if !strings.Contains(string(data), "name: testbot") {
		t.Error("agent.md not recreated by force init")
	}
}

func TestInitConfigFile(t *testing.T) {
	root := t.TempDir()
	agent := "testbot"

	Init(root, agent, false)

	configPath := filepath.Join(root, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config.yaml not found: %v", err)
	}
	if !strings.Contains(string(data), "agent: testbot") {
		t.Error("config.yaml does not contain agent name")
	}
}
