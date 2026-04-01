package scaffold

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SeanoChang/cubit/internal/config"
	"gopkg.in/yaml.v3"
)

// Init creates the v1.0 agent workspace at root/agent.
// Returns (true, nil) if created, (false, nil) if already exists.
// If force is true, recreates .claude/ config files for an existing workspace.
func Init(root, agent string, force bool) (bool, error) {
	agentDir := filepath.Join(root, "agents-home", agent)

	if _, err := os.Stat(agentDir); err == nil {
		if !force {
			return false, nil
		}
	}

	// Directories
	dirs := []string{
		filepath.Join(agentDir, "scratch"),
		filepath.Join(agentDir, "projects"),
		filepath.Join(agentDir, "memory"),
		filepath.Join(agentDir, "mailbox", "inbox", "important"),
		filepath.Join(agentDir, "mailbox", "inbox", "priority"),
		filepath.Join(agentDir, "mailbox", "inbox", "all"),
		filepath.Join(agentDir, "mailbox", "starred"),
		filepath.Join(agentDir, "mailbox", "drafts"),
		filepath.Join(agentDir, "mailbox", "sent"),
		filepath.Join(agentDir, "mailbox", "read"),
		filepath.Join(agentDir, ".claude", "agents"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return false, err
		}
	}

	// Template files
	files := map[string]string{
		filepath.Join(agentDir, "FLUCTLIGHT.md"): "# Identity\n\n<!-- Define your agent's identity here -->\n",
		filepath.Join(agentDir, "PROGRAM.md"): "# Program\n\nRead GOALS.md. Work on the highest-priority incomplete goal.\nWhen you've made meaningful progress on one goal, update MEMORY.md and log.md, then exit.\nIf a goal is fully complete, remove it from GOALS.md before exiting.\n",
		filepath.Join(agentDir, "GOALS.md"):  "# Goals\n\n<!-- Add goals here. Agent removes completed goals. -->\n",
		filepath.Join(agentDir, "MEMORY.md"): "# Memory\n\n<!-- Agent-maintained working context. Updated between sessions. -->\n",
		filepath.Join(agentDir, "log.md"):    "# Log\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return false, err
		}
	}

	// .claude/settings.json — default permissions, user can edit afterward
	settings := `{
  "permissions": {
    "allow": [
      "Read(**)",
      "Glob",
      "Grep",
      "Agent",
      "Write(*)",
      "Edit(*)",
      "Bash(git *)",
      "Bash(nark *)",
      "Bash(cubit *)",
      "Bash(mkdir *)",
      "Bash(ls *)",
      "Bash(cat *)",
      "Bash(head *)",
      "Bash(tail *)",
      "Bash(wc *)",
      "Bash(grep *)",
      "Bash(find *)",
      "Bash(rg *)"
    ],
    "deny": [
      "Bash(rm -rf *)",
      "Bash(sudo *)",
      "Bash(curl *)",
      "Bash(wget *)"
    ]
  }
}
`
	settingsPath := filepath.Join(agentDir, ".claude", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(settings), 0o644); err != nil {
		return false, err
	}

	// .claude/agents/<agent>.md
	agentMD := fmt.Sprintf(`---
name: %s
description: Agent workspace managed by cubit.
tools: Agent, Read, Write, Edit, Bash, Grep, Glob
---

# Boot Protocol

1. Read FLUCTLIGHT.md — this is your identity. Never modify it.
2. Read GOALS.md — these are your current objectives.
3. Read MEMORY.md — this is your working context from previous sessions.
4. Check mailbox: list mailbox/inbox/ for unread messages. Triage — add actionable items to GOALS.md, move to mailbox/starred/ or mailbox/read/.
5. For each goal, determine if it relates to an existing project:
   - Run `+"`cubit project search <keywords>`"+` to find related work
   - If found: cd into the project directory, review recent commits, continue
   - If not found: run `+"`cubit project new <name>`"+` to create a fresh project
6. Work in the project directory. Commit at natural checkpoints.
7. When you complete meaningful work:
   - Update MEMORY.md with current state
   - Append a one-line summary to log.md
   - If a goal is fully complete, remove it from GOALS.md
8. Write deliverables to DELIVER.md.
`, agent)
	agentMDPath := filepath.Join(agentDir, ".claude", "agents", agent+".md")
	if err := os.WriteFile(agentMDPath, []byte(agentMD), 0o644); err != nil {
		return false, err
	}

	// Write config.yaml at root if it doesn't exist
	configPath := filepath.Join(root, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := config.Default(agent)
		data, merr := yaml.Marshal(cfg)
		if merr != nil {
			return false, merr
		}
		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			return false, err
		}
	}

	return true, nil
}
