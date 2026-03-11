# Cubit v1.0 Remake — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strip Cubit from a ~7000-line custom runtime (DAG executor, MCP server, memory tiers, worker spawning) down to a ~200-line filesystem CLI that scaffolds agent workspaces and delegates execution to Claude Code's native subagents.

**Architecture:** Cubit becomes a filesystem layout manager. It scaffolds agent directories under `~/.ark/<agent>/`, reports status, and archives to nark. Claude Code handles execution (subagents, parallelism, context). Keel handles the outer loop (separate project). Each agent workspace is its own git repo with `.claude/` config for autonomous operation.

**Tech Stack:** Go 1.26, Cobra, Viper, `os/exec` (for `git init` and `nark write`)

**Source spec:** Notion — "Cubit v1.0 — Remake" (page 32030938db7c8143a68ac51795e6b89b)

---

## File Structure

### Files to DELETE (entire directories)

```
cmd/task/           # 507 lines — task management (todo, queue, do, done, requeue, log, graph)
cmd/exec/           # 481 lines — execution (prompt, run, summarize)
cmd/agent/          # 344 lines — agent state (identity, memory, status, brief, refresh)
cmd/mcp/            # 37 lines  — MCP server command
cmd/config.go       # 27 lines  — config show command
internal/queue/     # 2272 lines — task state machine, DAG, executor
internal/brief/     # 1113 lines — session brief assembly, memory pass, lifecycle
internal/mcp/       # 1099 lines — MCP stdio server, 9 tools
internal/claude/    # 113 lines  — claude -p wrapper
internal/scaffold/setup.go  # 229 lines — interactive LLM onboarding
```

### Files to KEEP as-is

```
main.go                     # 8 lines  — entry point
cmd/version.go              # 21 lines — version command
cmd/update.go               # 29 lines — self-update command
internal/updater/updater.go # 153 lines — GitHub release updater
internal/updater/updater_test.go # 106 lines
```

### Files to MODIFY

```
cmd/root.go                  # strip old imports/registrations, wire new commands
internal/config/config.go    # remove ClaudeConfig, change root to ~/.ark
internal/scaffold/scaffold.go # new filesystem layout
```

### Files to CREATE

```
cmd/init.go                  # rewrite — simpler init for new scaffold
cmd/status.go                # new — show goals, memory tokens, log tail
cmd/edit.go                  # new — open agent files in $EDITOR
cmd/archive.go               # new — archive scratch+log to nark, clean up
cmd/migrate.go               # new — migrate v0.x workspace to v1.0 layout
internal/config/config.go    # rewrite — simplified config with migration detection
internal/config/config_test.go  # new — config tests
internal/scaffold/scaffold_test.go # new — scaffold tests
```

### Final file tree after v1.0

```
main.go
cmd/
  root.go           # ~40 lines — root command, wires init/status/edit/archive/version/update
  version.go        # 21 lines  — unchanged
  update.go         # 29 lines  — unchanged
  init.go           # ~50 lines — scaffold new agent workspace
  status.go         # ~60 lines — show goals, memory tokens, log tail
  edit.go           # ~40 lines — open agent files in $EDITOR
  archive.go        # ~60 lines — archive to nark, clean scratch
  migrate.go        # ~100 lines — migrate v0.x workspace to v1.0 layout
internal/
  config/
    config.go       # ~60 lines — simplified config + old layout detection
    config_test.go  # ~40 lines
  scaffold/
    scaffold.go     # ~120 lines — new layout with git init + .claude/ + templates
    scaffold_test.go # ~80 lines
  updater/
    updater.go      # 153 lines — unchanged
    updater_test.go # 106 lines — unchanged
```

**Estimated total: ~1000 lines** (production + tests). Down from ~7000.

---

## Chunk 1: Strip Old Code + Simplify Config

> **IMPORTANT:** Tasks 1-3 are a single atomic operation. The project cannot compile between
> them because root.go references deleted packages and config.go imports deleted internal/claude.
> All three changes must land in one commit.

### Task 1: Delete old code + rewrite root.go + simplify config + stub init.go

Remove all v0.5 packages, rewrite root.go and config.go to remove references to deleted
code, and stub init.go to remove the `scaffold.RunSetup` call. Everything compiles after this.

**Files:**
- Delete: `cmd/task/`, `cmd/exec/`, `cmd/agent/`, `cmd/mcp/`, `cmd/config.go`
- Delete: `internal/queue/`, `internal/brief/`, `internal/mcp/`, `internal/claude/`
- Delete: `internal/scaffold/setup.go`
- Rewrite: `cmd/root.go`, `cmd/init.go`, `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Delete old directories and files**

```bash
cd /Users/seanochang/dev/projects/agents/cubit
rm -rf cmd/task cmd/exec cmd/agent cmd/mcp
rm cmd/config.go
rm -rf internal/queue internal/brief internal/mcp internal/claude
rm internal/scaffold/setup.go
```

- [ ] **Step 2: Verify deletions**

```bash
ls cmd/
# Expected: init.go  root.go  update.go  version.go
ls internal/
# Expected: config  scaffold  updater
```

- [ ] **Step 3: Rewrite root.go**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/SeanoChang/cubit/internal/config"
	"github.com/spf13/cobra"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:     "cubit",
	Short:   "Filesystem CLI for agent workspaces",
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("cubit %s (commit: %s, built: %s)\n", Version, Commit, Date))

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(initCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Stub init.go (remove RunSetup, will be fully rewritten in Task 5)**

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/SeanoChang/cubit/internal/scaffold"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [agent_name]",
	Short: "Scaffold a new agent workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var agent string
		if len(args) > 0 {
			agent = args[0]
		} else {
			fmt.Print("Agent name: ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				agent = strings.TrimSpace(scanner.Text())
			}
			if agent == "" {
				return fmt.Errorf("agent name is required")
			}
		}

		created, err := scaffold.Init(cfg.Root, agent, false)
		if err != nil {
			return fmt.Errorf("initializing agent: %w", err)
		}
		if created {
			fmt.Printf("Initialized agent %q\n", agent)
		} else {
			fmt.Printf("Agent %q already exists\n", agent)
		}
		return nil
	},
}
```

- [ ] **Step 5: Rewrite config.go (remove ClaudeConfig, change root to ~/.ark)**

This is the same config.go content as shown in Task 3 below.

- [ ] **Step 6: Update scaffold.go Init() signature to accept force parameter**

Change the function signature in `internal/scaffold/scaffold.go` from:
```go
func Init(root, agent string) (bool, error) {
```
to:
```go
func Init(root, agent string, force bool) (bool, error) {
```

And update the existing-directory check accordingly (see Task 4 for full implementation).
For now, a minimal change to make it compile:

```go
func Init(root, agent string, force bool) (bool, error) {
	agentDir := filepath.Join(root, agent)
	if _, err := os.Stat(agentDir); err == nil {
		if !force {
			return false, nil
		}
	}
	// ... rest stays the same for now (will be fully rewritten in Task 4)
```

- [ ] **Step 7: Verify build compiles**

```bash
go build -o cubit .
./cubit version
# Expected: cubit dev (commit: none, built: unknown)
```

- [ ] **Step 8: Commit (single atomic commit)**

```bash
git add -A
git commit -m "refactor: strip v0.5 code, simplify to filesystem CLI skeleton

- Delete ~6000 lines: task, exec, agent, mcp, queue, brief, claude
- Rewrite root.go: only version, update, init remain
- Simplify config: remove ClaudeConfig, root → ~/.ark
- Stub init.go: remove RunSetup dependency
Preparing for v1.0 filesystem CLI rebuild."
```

---

### Task 2: Config tests (now that everything compiles)

**Files:**
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write config tests**

```go
// internal/config/config_test.go
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
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/config/ -v
# Expected: PASS (config.go was already rewritten in Task 1)
```

- [ ] **Step 3: Commit**

```bash
git add internal/config/config_test.go
git commit -m "test: add config tests for DefaultRoot, Default, AgentDir"
```

---

## Chunk 2: Scaffold + Init

### Task 3: Rewrite scaffold

New `Init()` creates the v1.0 filesystem layout: git repo, .claude/ config, template files.

**Files:**
- Modify: `internal/scaffold/scaffold.go`
- Create: `internal/scaffold/scaffold_test.go`

- [ ] **Step 1: Write failing scaffold test**

```go
// internal/scaffold/scaffold_test.go
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

	agentDir := filepath.Join(root, agent)

	// Check directories exist
	dirs := []string{
		"scratch",
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
		".gitignore",
		".claude/settings.json",
		".claude/agents/testbot.md",
	}
	for _, f := range files {
		path := filepath.Join(agentDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("file %s not found: %v", f, err)
		}
	}

	// Check .git/ exists (git init ran)
	gitDir := filepath.Join(agentDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Errorf(".git/ not found — git init did not run: %v", err)
	}

	// Check agent.md contains agent name
	agentMD, _ := os.ReadFile(filepath.Join(agentDir, ".claude", "agents", "testbot.md"))
	if !strings.Contains(string(agentMD), "name: testbot") {
		t.Error("agent.md does not contain agent name")
	}

	// Check scratch/.gitkeep exists
	gitkeep := filepath.Join(agentDir, "scratch", ".gitkeep")
	if _, err := os.Stat(gitkeep); err != nil {
		t.Errorf("scratch/.gitkeep not found: %v", err)
	}
}

func TestInitAlreadyExists(t *testing.T) {
	root := t.TempDir()
	agent := "testbot"

	// First init
	Init(root, agent, false)

	// Second init (no force)
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

	// First init
	Init(root, agent, false)

	// Modify settings.json to verify force recreates it
	settingsPath := filepath.Join(root, agent, ".claude", "settings.json")
	os.WriteFile(settingsPath, []byte("corrupted"), 0o644)

	// Force re-init
	created, err := Init(root, agent, true)
	if err != nil {
		t.Fatalf("Init(force=true) error: %v", err)
	}
	if !created {
		t.Fatal("Init(force=true) returned created=false, want true")
	}

	// Verify settings.json was recreated
	data, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(data), "dontAsk") {
		t.Error("settings.json not recreated by force init")
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/scaffold/ -v
# Expected: FAIL — old scaffold creates different layout
```

- [ ] **Step 3: Rewrite scaffold.go**

```go
package scaffold

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/SeanoChang/cubit/internal/config"
	"gopkg.in/yaml.v3"
)

// Init creates the v1.0 agent workspace at root/agent.
// Returns (true, nil) if created, (false, nil) if already exists.
// If force is true, recreates .claude/ and template files for an existing workspace.
func Init(root, agent string, force bool) (bool, error) {
	agentDir := filepath.Join(root, agent)

	if _, err := os.Stat(agentDir); err == nil {
		if !force {
			return false, nil
		}
		// Force mode: recreate .claude/ config files (preserves user data files)
	}

	// Directories
	dirs := []string{
		filepath.Join(agentDir, "scratch"),
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
		filepath.Join(agentDir, "PROGRAM.md"): `# Program

Read GOALS.md. Work on the highest-priority incomplete goal.
When you've made meaningful progress on one goal, update MEMORY.md and log.md, then exit.
If a goal is fully complete, remove it from GOALS.md before exiting.
`,
		filepath.Join(agentDir, "GOALS.md"):  "# Goals\n\n<!-- Add goals here. Agent removes completed goals. -->\n",
		filepath.Join(agentDir, "MEMORY.md"): "# Memory\n\n<!-- Agent-maintained working context. Updated between sessions. -->\n",
		filepath.Join(agentDir, "log.md"):    "# Log\n",
		filepath.Join(agentDir, "scratch", ".gitkeep"): "",
		filepath.Join(agentDir, ".gitignore"): "scratch/*\n!scratch/.gitkeep\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return false, err
		}
	}

	// .claude/settings.json
	settings := `{
  "permissions": {
    "defaultMode": "dontAsk",
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

1. Read ~/.ark/%s/FLUCTLIGHT.md — this is your identity. Never modify it.
2. Read ~/.ark/%s/GOALS.md — these are your current objectives.
3. Read ~/.ark/%s/MEMORY.md — this is your working context from previous sessions.
4. Work on the highest-priority incomplete goal.
5. Use subagents for parallel research or independent subtasks.
6. When you complete meaningful work:
   - Update MEMORY.md with current state
   - Append a one-line summary to log.md
   - If a goal is fully complete, remove it from GOALS.md
7. Write working files to scratch/<task-name>/.
`, agent, agent, agent, agent)
	agentMDPath := filepath.Join(agentDir, ".claude", "agents", agent+".md")
	if err := os.WriteFile(agentMDPath, []byte(agentMD), 0o644); err != nil {
		return false, err
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = agentDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("git init: %w", err)
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/scaffold/ -v
# Expected: PASS (all 3 tests)
```

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/scaffold.go internal/scaffold/scaffold_test.go
git commit -m "feat: rewrite scaffold for v1.0 filesystem layout

New layout: FLUCTLIGHT.md, PROGRAM.md, GOALS.md, MEMORY.md, log.md,
scratch/, .claude/agents/<agent>.md, .claude/settings.json, git init.
No more queue/, state.json, memory/sessions/."
```

---

### Task 4: Rewrite init command

Simplified init that uses the new scaffold. Keeps `--import-identity` and `--force`.

**Files:**
- Modify: `cmd/init.go`

- [ ] **Step 1: Rewrite init.go**

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SeanoChang/cubit/internal/scaffold"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [agent_name]",
	Short: "Scaffold a new agent workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var agent string
		if len(args) > 0 {
			agent = args[0]
		} else {
			fmt.Print("Agent name: ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				agent = strings.TrimSpace(scanner.Text())
			}
			if agent == "" {
				return fmt.Errorf("agent name is required")
			}
		}

		root := cfg.Root
		agentDir := filepath.Join(root, agent)
		force, _ := cmd.Flags().GetBool("force")

		created, err := scaffold.Init(root, agent, force)
		if err != nil {
			return fmt.Errorf("initializing agent: %w", err)
		}

		if !created {
			fmt.Printf("Agent %q already exists at %s (use --force to re-initialize)\n", agent, agentDir)
			return nil
		}

		if force {
			fmt.Printf("Re-initialized agent %q at %s\n", agent, agentDir)
		} else {
			fmt.Printf("Initialized agent %q at %s\n", agent, agentDir)
		}

		// --import-identity FILE: copy an existing FLUCTLIGHT.md
		importPath, _ := cmd.Flags().GetString("import-identity")
		if importPath != "" {
			data, err := os.ReadFile(importPath)
			if err != nil {
				return fmt.Errorf("reading identity file: %w", err)
			}
			dest := filepath.Join(agentDir, "FLUCTLIGHT.md")
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return err
			}
			fmt.Printf("  imported %s → FLUCTLIGHT.md\n", importPath)
		}

		return nil
	},
}

func init() {
	initCmd.Flags().String("import-identity", "", "Import an existing FLUCTLIGHT.md file")
	initCmd.Flags().Bool("force", false, "Re-initialize an existing agent workspace")
}
```

- [ ] **Step 2: Update root.go to remove init flag registration (now in init.go's own init())**

In `cmd/root.go`, remove the init flag lines from `init()`:

```go
// Remove these lines from root.go init():
// initCmd.Flags().Bool("skip-onboard", false, "...")
// initCmd.Flags().String("import-identity", "", "...")
// initCmd.Flags().Bool("force", false, "...")
```

The `init()` in `cmd/root.go` should just be:

```go
func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("cubit %s (commit: %s, built: %s)\n", Version, Commit, Date))

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(initCmd)
}
```

- [ ] **Step 3: Verify build and test init**

```bash
go build -o cubit .
./cubit init --help
# Expected: shows init usage with --import-identity and --force flags
```

- [ ] **Step 4: Commit**

```bash
git add cmd/init.go cmd/root.go
git commit -m "feat: rewrite init command for v1.0 layout

Simplified init — scaffolds agent workspace with new layout.
Flags: --import-identity, --force. No more --skip-onboard."
```

---

## Chunk 3: Migration

### Task 5: Add old layout detection to config

When cubit runs against an old v0.x workspace (`~/.ark/cubit/<agent>/`), it should detect this and warn the user.

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add detection function to config.go**

After the existing `AgentDir()` method, add:

```go
// IsLegacyLayout returns true if the old v0.x layout exists at ~/.ark/cubit/<agent>/.
// Old layout had identity/, queue/, memory/sessions/ subdirectories.
func (c *Config) IsLegacyLayout() bool {
	oldRoot := filepath.Join(filepath.Dir(c.Root), ".ark", "cubit")
	if c.Root == oldRoot {
		// Config already points to old root — definitely legacy
		return true
	}
	oldAgentDir := filepath.Join(oldRoot, c.Agent)
	_, err := os.Stat(filepath.Join(oldAgentDir, "identity"))
	return err == nil
}

// LegacyAgentDir returns the old v0.x agent directory path.
func (c *Config) LegacyAgentDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ark", "cubit", c.Agent)
}
```

- [ ] **Step 2: Add legacy config file fallback to Load()**

Update `Load()` to also check the old config path `~/.ark/cubit/config.yaml`:

```go
func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("agent", "noah")
	v.SetDefault("root", DefaultRoot())

	root := DefaultRoot()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(root)

	// Also check old v0.x config location
	home, _ := os.UserHomeDir()
	oldRoot := filepath.Join(home, ".ark", "cubit")
	v.AddConfigPath(oldRoot)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Normalize root — if it's the old path, update to new default
	if cfg.Root == oldRoot {
		cfg.Root = DefaultRoot()
	}
	if cfg.Root == "~/.ark" {
		cfg.Root = DefaultRoot()
	}

	return &cfg, nil
}
```

- [ ] **Step 3: Add startup warning to root.go PersistentPreRunE**

After loading config, check for legacy layout:

```go
PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
	if cmd.Name() == "version" || cmd.Name() == "help" {
		return nil
	}
	var err error
	cfg, err = config.Load()
	if err != nil {
		return err
	}

	// Warn if old v0.x layout detected
	if cmd.Name() != "migrate" && cfg.IsLegacyLayout() {
		fmt.Fprintf(os.Stderr, "⚠ Legacy v0.x workspace detected at %s\n", cfg.LegacyAgentDir())
		fmt.Fprintf(os.Stderr, "  Run 'cubit migrate' to upgrade to v1.0 layout.\n\n")
	}

	return nil
},
```

- [ ] **Step 4: Verify build**

```bash
go build -o cubit .
```

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go cmd/root.go
git commit -m "feat: detect legacy v0.x layout and warn on startup"
```

---

### Task 6: Migrate command

Migrate v0.x agent workspace to v1.0 layout. Moves files, creates new structure, backs up old data.

**Files:**
- Create: `cmd/migrate.go`

- [ ] **Step 1: Create migrate.go**

```go
package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SeanoChang/cubit/internal/scaffold"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate v0.x workspace to v1.0 layout",
	Long:  "Moves agent data from ~/.ark/cubit/<agent>/ to ~/.ark/<agent>/ with new filesystem layout.",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := cfg.Agent
		home, _ := os.UserHomeDir()
		oldAgentDir := filepath.Join(home, ".ark", "cubit", agent)
		newAgentDir := cfg.AgentDir() // ~/.ark/<agent>

		// Check old layout exists
		if _, err := os.Stat(oldAgentDir); os.IsNotExist(err) {
			return fmt.Errorf("no v0.x workspace found at %s", oldAgentDir)
		}

		// Check new layout doesn't already exist
		if _, err := os.Stat(newAgentDir); err == nil {
			return fmt.Errorf("v1.0 workspace already exists at %s — nothing to migrate", newAgentDir)
		}

		fmt.Printf("Migrating %q: %s → %s\n\n", agent, oldAgentDir, newAgentDir)

		// Step 1: Create new workspace via scaffold
		created, err := scaffold.Init(cfg.Root, agent, false)
		if err != nil {
			return fmt.Errorf("creating v1.0 workspace: %w", err)
		}
		if !created {
			return fmt.Errorf("failed to create v1.0 workspace")
		}

		// Step 2: Migrate user data files
		migrations := []struct {
			src  string
			dst  string
			desc string
		}{
			{
				filepath.Join(oldAgentDir, "identity", "FLUCTLIGHT.md"),
				filepath.Join(newAgentDir, "FLUCTLIGHT.md"),
				"FLUCTLIGHT.md (identity)",
			},
			{
				filepath.Join(oldAgentDir, "GOALS.md"),
				filepath.Join(newAgentDir, "GOALS.md"),
				"GOALS.md",
			},
			{
				filepath.Join(oldAgentDir, "memory", "MEMORY.md"),
				filepath.Join(newAgentDir, "MEMORY.md"),
				"MEMORY.md",
			},
			{
				filepath.Join(oldAgentDir, "memory", "log.md"),
				filepath.Join(newAgentDir, "log.md"),
				"log.md",
			},
		}

		for _, m := range migrations {
			if err := copyFileIfExists(m.src, m.dst); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: %s — %v\n", m.desc, err)
			} else if fileExists(m.src) {
				fmt.Printf("  ✓ %s\n", m.desc)
			}
		}

		// Step 3: Merge brief.md into MEMORY.md if it has content
		briefPath := filepath.Join(oldAgentDir, "memory", "brief.md")
		if briefData, err := os.ReadFile(briefPath); err == nil && len(briefData) > 0 {
			memoryPath := filepath.Join(newAgentDir, "MEMORY.md")
			f, err := os.OpenFile(memoryPath, os.O_APPEND|os.O_WRONLY, 0o644)
			if err == nil {
				f.WriteString("\n## Working Context (migrated from brief.md)\n\n")
				f.Write(briefData)
				f.Close()
				fmt.Println("  ✓ brief.md → merged into MEMORY.md")
			}
		}

		// Step 4: Copy scratch/ contents (if any)
		oldScratch := filepath.Join(oldAgentDir, "scratch")
		newScratch := filepath.Join(newAgentDir, "scratch")
		if entries, err := os.ReadDir(oldScratch); err == nil {
			count := 0
			for _, entry := range entries {
				src := filepath.Join(oldScratch, entry.Name())
				dst := filepath.Join(newScratch, entry.Name())
				if entry.IsDir() {
					copyDir(src, dst)
				} else {
					copyFileIfExists(src, dst)
				}
				count++
			}
			if count > 0 {
				fmt.Printf("  ✓ scratch/ (%d items)\n", count)
			}
		}

		// Step 5: Write new config at ~/.ark/config.yaml
		newConfigPath := filepath.Join(cfg.Root, "config.yaml")
		if _, err := os.Stat(newConfigPath); os.IsNotExist(err) {
			configContent := fmt.Sprintf("agent: %s\nroot: %s\n", agent, cfg.Root)
			os.WriteFile(newConfigPath, []byte(configContent), 0o644)
			fmt.Println("  ✓ config.yaml")
		}

		// Step 6: Rename old directory as backup
		backupDir := oldAgentDir + ".v0-backup"
		if err := os.Rename(oldAgentDir, backupDir); err != nil {
			fmt.Fprintf(os.Stderr, "\n  warning: could not rename old workspace: %v\n", err)
			fmt.Fprintf(os.Stderr, "  old workspace remains at %s\n", oldAgentDir)
		} else {
			fmt.Printf("\n  Old workspace backed up to %s\n", backupDir)
		}

		fmt.Printf("\nMigration complete. Run 'cubit status' to verify.\n")
		return nil
	},
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFileIfExists(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return nil // source doesn't exist, skip silently
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFileIfExists(path, target)
	})
}
```

- [ ] **Step 2: Register in root.go**

Add to `cmd/root.go` `init()`:

```go
rootCmd.AddCommand(migrateCmd)
```

- [ ] **Step 3: Verify build**

```bash
go build -o cubit .
./cubit migrate --help
# Expected: shows migrate usage
```

- [ ] **Step 4: Commit**

```bash
git add cmd/migrate.go cmd/root.go
git commit -m "feat: add migrate command — v0.x to v1.0 workspace upgrade

Moves agent data from ~/.ark/cubit/<agent>/ to ~/.ark/<agent>/:
- identity/FLUCTLIGHT.md → FLUCTLIGHT.md
- memory/MEMORY.md → MEMORY.md
- memory/brief.md → merged into MEMORY.md
- memory/log.md → log.md
- scratch/ contents preserved
- Old workspace backed up to <dir>.v0-backup"
```

---

## Chunk 4: Commands + Cleanup

### Task 7: Status command

Show GOALS.md content, MEMORY.md token estimate, and log.md tail.

**Files:**
- Create: `cmd/status.go`

- [ ] **Step 1: Create status.go**

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show agent workspace status",
	RunE: func(cmd *cobra.Command, args []string) error {
		agentDir := cfg.AgentDir()
		fmt.Printf("Agent: %s\n", cfg.Agent)
		fmt.Printf("Path:  %s\n\n", agentDir)

		// GOALS.md
		goalsPath := filepath.Join(agentDir, "GOALS.md")
		goals, err := os.ReadFile(goalsPath)
		if err != nil {
			fmt.Println("Goals: (not found)")
		} else {
			content := strings.TrimSpace(string(goals))
			if content == "" || content == "# Goals" {
				fmt.Println("Goals: (none)")
			} else {
				fmt.Printf("Goals:\n%s\n", content)
			}
		}
		fmt.Println()

		// MEMORY.md token estimate
		memoryPath := filepath.Join(agentDir, "MEMORY.md")
		memory, err := os.ReadFile(memoryPath)
		if err != nil {
			fmt.Println("Memory: (not found)")
		} else {
			words := len(strings.Fields(string(memory)))
			tokens := int(float64(words) * 1.3)
			fmt.Printf("Memory: ~%d tokens (%d words)\n", tokens, words)
		}

		// log.md tail
		logPath := filepath.Join(agentDir, "log.md")
		logData, err := os.ReadFile(logPath)
		if err != nil {
			fmt.Println("Log:    (not found)")
		} else {
			lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
			n := 5
			if len(lines) < n {
				n = len(lines)
			}
			tail := lines[len(lines)-n:]
			fmt.Printf("Log (last %d lines):\n", n)
			for _, line := range tail {
				fmt.Printf("  %s\n", line)
			}
		}

		return nil
	},
}
```

- [ ] **Step 2: Register in root.go**

Add to `cmd/root.go` `init()`:

```go
rootCmd.AddCommand(statusCmd)
```

- [ ] **Step 3: Verify build**

```bash
go build -o cubit .
./cubit status --help
# Expected: shows status usage
```

- [ ] **Step 4: Commit**

```bash
git add cmd/status.go cmd/root.go
git commit -m "feat: add status command — goals, memory tokens, log tail"
```

---

### Task 8: Edit command

Open agent files in `$EDITOR`. Supports: goals, memory, program, fluctlight.

**Files:**
- Create: `cmd/edit.go`

- [ ] **Step 1: Create edit.go**

```go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var targets = map[string]string{
	"goals":      "GOALS.md",
	"memory":     "MEMORY.md",
	"program":    "PROGRAM.md",
	"fluctlight": "FLUCTLIGHT.md",
}

var editCmd = &cobra.Command{
	Use:       "edit <target>",
	Short:     "Open an agent file in $EDITOR",
	Long:      "Targets: goals, memory, program, fluctlight",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"goals", "memory", "program", "fluctlight"},
	RunE: func(cmd *cobra.Command, args []string) error {
		filename, ok := targets[args[0]]
		if !ok {
			return fmt.Errorf("unknown target %q — use: goals, memory, program, fluctlight", args[0])
		}

		path := filepath.Join(cfg.AgentDir(), filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("%s not found at %s", filename, path)
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		c := exec.Command(editor, path)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}
```

- [ ] **Step 2: Register in root.go**

Add to `cmd/root.go` `init()`:

```go
rootCmd.AddCommand(editCmd)
```

- [ ] **Step 3: Verify build**

```bash
go build -o cubit .
./cubit edit --help
# Expected: shows edit usage with target list
```

- [ ] **Step 4: Commit**

```bash
git add cmd/edit.go cmd/root.go
git commit -m "feat: add edit command — open agent files in \$EDITOR"
```

---

### Task 9: Archive command

Collect scratch/ files, write to nark, clean scratch/, append log entry.

**Files:**
- Create: `cmd/archive.go`

- [ ] **Step 1: Create archive.go**

```go
package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive scratch + log to nark, clean scratch",
	RunE: func(cmd *cobra.Command, args []string) error {
		agentDir := cfg.AgentDir()
		scratchDir := filepath.Join(agentDir, "scratch")

		// Walk scratch/ recursively to collect all files (including subdirs)
		var content strings.Builder
		content.WriteString(fmt.Sprintf("# Archive — %s\n\n", cfg.Agent))

		fileCount := 0
		filepath.WalkDir(scratchDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() == ".gitkeep" {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			relPath, _ := filepath.Rel(scratchDir, path)
			content.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", relPath, string(data)))
			fileCount++
			return nil
		})

		if fileCount == 0 {
			fmt.Println("scratch/ is empty — nothing to archive.")
			return nil
		}

		// Try nark write — abort cleanup if nark fails
		date := time.Now().Format("2006-01-02")
		title := fmt.Sprintf("cubit archive %s %s", cfg.Agent, date)

		narkCmd := exec.Command("nark", "write", "--title", title)
		narkCmd.Stdin = strings.NewReader(content.String())
		output, err := narkCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("nark write failed: %v\nscratch/ not cleaned — archive manually or install nark", err)
		}

		narkID := strings.TrimSpace(string(output))
		fmt.Printf("Archived %d files to nark: %s\n", fileCount, narkID)

		// Append log entry
		logPath := filepath.Join(agentDir, "log.md")
		logEntry := fmt.Sprintf("\n[%s] Archived %d files to nark: %s\n", date, fileCount, narkID)
		f, logErr := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if logErr == nil {
			f.WriteString(logEntry)
			f.Close()
		}

		// Clean scratch/ (keep .gitkeep, remove everything else)
		entries, _ := os.ReadDir(scratchDir)
		for _, entry := range entries {
			if entry.Name() == ".gitkeep" {
				continue
			}
			path := filepath.Join(scratchDir, entry.Name())
			if entry.IsDir() {
				os.RemoveAll(path)
			} else {
				os.Remove(path)
			}
		}
		fmt.Println("Cleaned scratch/")

		return nil
	},
}
```

- [ ] **Step 2: Register in root.go**

Add to `cmd/root.go` `init()`:

```go
rootCmd.AddCommand(archiveCmd)
```

- [ ] **Step 3: Verify build**

```bash
go build -o cubit .
./cubit archive --help
# Expected: shows archive usage
```

- [ ] **Step 4: Commit**

```bash
git add cmd/archive.go cmd/root.go
git commit -m "feat: add archive command — scratch to nark, clean up"
```

---

### Task 10: Clean go.mod + final verification

Remove unused dependencies and verify everything builds and tests pass.

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Remove golang.org/x/sync (was for parallel execution semaphore)**

```bash
go mod tidy
```

- [ ] **Step 2: Verify build**

```bash
go build -o cubit .
./cubit version
./cubit --help
# Expected: shows init, migrate, status, edit, archive, version, update, help
```

- [ ] **Step 3: Run all tests**

```bash
go test ./... -v
# Expected: PASS for config, scaffold, updater
```

- [ ] **Step 4: Verify command tree**

```bash
./cubit --help
# Expected output:
# Filesystem CLI for agent workspaces
#
# Usage:
#   cubit [command]
#
# Available Commands:
#   archive     Archive scratch + log to nark, clean scratch
#   edit        Open an agent file in $EDITOR
#   help        Help about any command
#   init        Scaffold a new agent workspace
#   migrate     Migrate v0.x workspace to v1.0 layout
#   status      Show agent workspace status
#   update      Update cubit to the latest release
#   version     Print cubit version
```

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: go mod tidy — remove unused dependencies"
```

- [ ] **Step 6: Update CLAUDE.md**

Update the project's CLAUDE.md to reflect the new structure:

```markdown
# Cubit

Agent workspace filesystem CLI. Go CLI built with Cobra + Viper.

## Build & Run

` ` `bash
go build -o cubit .
./cubit version
./cubit init noah
./cubit status
` ` `

## Project Layout

- `cmd/` — Cobra root + commands (init, migrate, status, edit, archive, version, update)
- `internal/config/` — Config types + loading via Viper
- `internal/scaffold/` — Agent workspace scaffolding
- `internal/updater/` — GitHub release self-updater
- `main.go` — Entry point

## Conventions

- Module: `github.com/SeanoChang/cubit`
- Config file: `~/.ark/config.yaml`
- Agent data: `~/.ark/<agent>/` (e.g. `~/.ark/noah/`)
- Each agent dir is a git repo with `.claude/` config
- Version injected via ldflags at build time
- Release targets: linux/amd64 + darwin/arm64
```

- [ ] **Step 7: Final commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for v1.0 filesystem CLI"
```

---

## Summary

| Metric | v0.5 | v1.0 |
|--------|------|------|
| Total lines | ~7000 | ~1000 |
| Commands | 15+ | 7 |
| Internal packages | 6 | 3 |
| Dependencies | 4 | 3 |
| MCP server | Yes | No |
| DAG executor | Yes | No |
| Claude runner | Yes | No |
| Memory tiers | 4 | 1 (MEMORY.md) |

**What cubit does:** scaffold, migrate, status, edit, archive.
**What Claude Code does:** execute, parallelize, manage context.
**What Keel does:** outer loop, Discord bridge.
