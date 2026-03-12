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
| `cubit status` | Show goals, memory token count, log tail |
| `cubit edit <target>` | Open agent file in `$EDITOR` (goals, memory, program, fluctlight, settings) |
| `cubit archive` | Push log + scratch to nark, truncate log, clean scratch |
| `cubit migrate [agents...]` | Migrate workspaces to agents-home layout |
| `cubit version` | Show version info |
| `cubit update` | Self-update from GitHub releases |

### Init Flags

```bash
cubit init noah                          # scaffold new workspace
cubit init noah --force                  # re-initialize existing workspace
cubit init noah --import-identity id.md  # import an existing FLUCTLIGHT.md
```

### Archive Flags

```bash
cubit archive                  # archive log + scratch to nark, keep last 50 log lines
cubit archive --keep-log 100   # keep last 100 log lines instead
```

## Agent Workspace Layout

Each agent workspace is a git repo under `~/.ark/agents-home/`:

```
~/.ark/agents-home/<agent>/
├── .git/
├── .gitignore
├── .claude/
│   ├── settings.json          # agent-specific permissions (user-editable)
│   └── agents/
│       └── <agent>.md         # agent definition + boot protocol
├── FLUCTLIGHT.md              # identity (immutable by agent)
├── PROGRAM.md                 # how to work (human-authored)
├── GOALS.md                   # what to work on (agent removes completed)
├── MEMORY.md                  # working context (agent-maintained)
├── log.md                     # append-only accomplishment history
└── scratch/                   # ephemeral workspace
    └── <task-name>/           # one dir per task
```

## Config

`~/.ark/config.yaml`:

```yaml
agent: noah
root: ~/.ark
```

## Migration from v0.x

Supports migrating from v0.x (`~/.ark/cubit/<agent>/`) or flat v1.0 (`~/.ark/<agent>/`):

```bash
cubit migrate noah scout    # migrate specific agents
cubit migrate               # migrate the default agent from config
```

Flat v1.0 workspaces are moved directly. V0.x workspaces are scaffolded fresh with data copied over and old directories backed up.

## Project Layout

```
cmd/                    # Cobra commands
  root.go               # Root command, wires all subcommands
  init.go               # Scaffold new agent workspace
  status.go             # Show workspace status
  edit.go               # Open agent files in $EDITOR
  archive.go            # Archive to nark, truncate log, clean scratch
  migrate.go            # v0.x → v1.0 migration
  version.go            # Version info
  update.go             # Self-update
internal/
  config/               # Config types + Viper loading
  scaffold/             # Agent workspace scaffolding
  updater/              # Self-update from GitHub releases
main.go                 # Entry point
```

## Build

```bash
go build -o cubit .
go test ./...
```

Version injected via ldflags at build time. Release targets: `linux/amd64` + `darwin/arm64`.
