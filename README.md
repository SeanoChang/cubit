# Cubit

Filesystem CLI for agent workspaces. Scaffolds directories, reports status, and archives to nark. Execution is handled by Claude Code's native subagents; the outer loop is Keel's job.

## Install

```bash
# From source
go install github.com/SeanoChang/cubit@latest

# Or build locally
go build -o cubit .

# Self-update
cubit update
```

## Quick Start

```bash
# Initialize a new agent workspace
cubit init noah

# Check workspace status
cubit status

# Edit goals
cubit edit goals

# Run the agent (via Claude Code)
cd ~/.ark/agents-home/noah && claude --agent noah -p "$(cat PROGRAM.md)"

# Archive scratch + log to nark
cubit archive
```

## Commands

| Command | Description |
|---------|-------------|
| `cubit init <agent>` | Scaffold agent workspace at `~/.ark/agents-home/<agent>/` |
| `cubit status` | Show goals, memory token count, projects, log tail |
| `cubit edit <target>` | Open agent file in `$EDITOR` (goals, memory, program, fluctlight, settings) |
| `cubit archive` | Push log + scratch to nark, truncate log, clean scratch |
| `cubit project new <name>` | Create a new project with git init |
| `cubit project list` | List all projects with commit count and last activity |
| `cubit project search <query>` | Search across all project repos |
| `cubit project archive <name>` | Archive a project to nark |
| `cubit project status <name>` | Show detailed project status |
| `cubit goal add <message>` | Add a typed goal to GOALS.md |
| `cubit goal ls` | List current goals with type tags |
| `cubit memory ls` | List memory topic files with line counts |
| `cubit memory check` | Show memory size stats (MEMORY.md + memory/) |
| `cubit dream` | LLM-powered memory consolidation into index + topic files |
| `cubit send <draft-file>` | Send a mailbox message to another agent |
| `cubit migrate [agents...]` | Migrate workspaces to agents-home layout |
| `cubit migrate-projects [agents...]` | Migrate git-at-root workspaces to projects/ layout |
| `cubit migrate-memory [agents...]` | Create memory/ directory for existing agents |
| `cubit migrate-mailbox [agents...]` | Create mailbox/ tree, migrate INBOX.md |
| `cubit version` | Show version info |
| `cubit update` | Self-update from GitHub releases |

### Init Flags

```bash
cubit init noah                          # scaffold new workspace
cubit init noah --force                  # re-initialize existing workspace
cubit init noah --import-identity id.md  # import an existing FLUCTLIGHT.md
```

### Goal Flags

```bash
cubit goal add "research market trends"                  # session goal (default)
cubit goal add --type background "check disk usage"      # background goal
cubit goal add --type consolidation "reorganize memory"  # consolidation goal
cubit goal add --from alice "review PR #42"              # set source
cubit goal ls                                            # list all goals
cubit goal ls --type background                          # filter by type
```

### Dream Flags

```bash
cubit dream                    # consolidate memory with Claude
cubit dream --dry-run          # show what would change without applying
cubit dream --include-log      # include log.md for temporal context
```

### Send

```bash
cubit send mailbox/drafts/my-message.md   # send a draft message
```

Draft files use YAML frontmatter:

```yaml
---
from: alice
to: noah
subject: Found a regression in auth module
category: important    # important | priority | all (default: all)
type: notification     # notification | request | handoff
---

Body of the message here.
```

### Archive Flags

```bash
cubit archive                  # archive log + scratch to nark, keep last 50 log lines
cubit archive --keep-log 100   # keep last 100 log lines instead
```

## Agent Workspace Layout

Each agent workspace is a plain directory under `~/.ark/agents-home/`. Git repos live inside `projects/`:

```
~/.ark/agents-home/<agent>/
├── .claude/
│   ├── settings.json          # agent-specific permissions (user-editable)
│   └── agents/
│       └── <agent>.md         # agent definition + boot protocol
├── FLUCTLIGHT.md              # identity (immutable by agent)
├── PROGRAM.md                 # how to work (human-authored)
├── GOALS.md                   # what to work on (agent removes completed)
├── MEMORY.md                  # working context (agent-maintained)
├── log.md                     # append-only accomplishment history
├── memory/                    # topic-based memory files (agent-maintained)
│   ├── <topic>.md             # e.g., architecture.md, decisions.md
│   └── archive/               # archived MEMORY.md snapshots from dream
├── mailbox/                   # inter-agent messaging
│   ├── inbox/
│   │   ├── important/         # high-priority messages
│   │   ├── priority/          # medium-priority messages
│   │   └── all/               # default category
│   ├── starred/               # starred messages
│   ├── drafts/                # outgoing drafts
│   ├── sent/                  # sent message copies
│   └── read/                  # processed messages
├── scratch/                   # ephemeral workspace
│   └── <task-name>/           # one dir per task
└── projects/                  # persistent versioned work
    ├── market-trends-q1/      # git repo
    │   ├── .git/
    │   ├── EVAL.md            # optional: evaluation rules
    │   └── <work files>
    └── trading-strategy/      # git repo
```

## Config

`~/.ark/config.yaml`:

```yaml
agent: noah
root: ~/.ark
```

## Migration

### From v0.x or flat v1.0

Supports migrating from v0.x (`~/.ark/cubit/<agent>/`) or flat v1.0 (`~/.ark/<agent>/`):

```bash
cubit migrate noah scout    # migrate specific agents
cubit migrate               # migrate the default agent from config
```

Flat v1.0 workspaces are moved directly. V0.x workspaces are scaffolded fresh with data copied over and old directories backed up.

### From git-at-root to projects/ layout

Workspaces created by older versions of `cubit init` have `.git` at the workspace root. To transition to the new model where only projects are git repos:

```bash
cubit migrate-projects noah    # migrate specific agents
cubit migrate-projects         # migrate the default agent
```

This moves `.git` into `projects/legacy/` (preserving history) or removes it if there are no commits. Workspace files (GOALS.md, MEMORY.md, etc.) are not moved.

## Project Layout

```
cmd/                    # Cobra commands
  root.go               # Root command, wires all subcommands
  helpers.go            # Shared utils (isValidAgentName)
  init.go               # Scaffold new agent workspace
  status.go             # Show workspace status
  edit.go               # Open agent files in $EDITOR
  archive.go            # Archive to nark, truncate log, clean scratch
  project.go            # Project management subcommands
  goal.go               # Typed goal management
  memory.go             # Memory topic file listing and stats
  dream.go              # LLM-powered memory consolidation
  send.go               # Inter-agent mailbox messaging
  migrate.go            # v0.x → v1.0 migration
  migrate_projects.go   # git-at-root → projects/ migration
  migrate_memory.go     # Create memory/ directory
  migrate_mailbox.go    # Create mailbox/ tree
  version.go            # Version info
  update.go             # Self-update
internal/
  config/               # Config types + Viper loading
  scaffold/             # Agent workspace scaffolding
  project/              # Project CRUD, search, git operations
  updater/              # Self-update from GitHub releases
main.go                 # Entry point
```

## Build

```bash
go build -o cubit .
go test ./...
```

Version injected via ldflags at build time. Release targets: `linux/amd64` + `darwin/arm64`.
