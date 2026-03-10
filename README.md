# Cubit

Control plane for a single AI agent instance. Manages identity, tasks, execution, and memory.

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
# Initialize a new agent
cubit init noah

# Create tasks
cubit todo "implement auth system"
cubit todo "write tests" --depends-on 1
cubit todo "architecture sweep" --mode loop --goal "val_bpb < 0.95" --max-iterations 100

# View the task graph
cubit graph

# Run the DAG executor
cubit run
```

## Commands

### Core

| Command | Description |
|---------|-------------|
| `cubit init <agent>` | Initialize agent workspace |
| `cubit config show` | Print current config |
| `cubit version` | Show version info |
| `cubit update` | Self-update from GitHub releases |

### Task Queue

| Command | Description |
|---------|-------------|
| `cubit todo <desc>` | Create a new task |
| `cubit queue` | List pending tasks |
| `cubit do [--all]` | Pop next ready task(s) |
| `cubit done [id] [summary]` | Complete a task |
| `cubit requeue [id]` | Return active task to queue |
| `cubit log <msg>` | Append observation to log |
| `cubit graph` | Print task DAG with status |

### Execution

| Command | Description |
|---------|-------------|
| `cubit prompt <msg>` | Single-shot prompt with brief injection |
| `cubit run` | Concurrent DAG executor |

### Observability

| Command | Description |
|---------|-------------|
| `cubit status` | Agent state, queue depth, brief size |
| `cubit brief` | Show brief sections + token estimates |
| `cubit refresh` | Rewrite brief.md from journals + log |

### Identity & Memory

| Command | Description |
|---------|-------------|
| `cubit identity list\|show\|set` | Manage identity files |
| `cubit memory show\|append\|edit` | Agent's durable notes |

## Task Modes

**once** — Single-shot execution with retry (default).

**loop** — Iterates until goal met or `max_iterations` reached. Each iteration re-injects `program.md`, runs a memory pass, and appends to `results.tsv`. The agent signals completion by outputting `GOAL_MET`.

```bash
cubit todo "optimize model" \
  --mode loop \
  --program sweep.md \
  --goal "val_bpb < 0.95" \
  --max-iterations 100 \
  --branch noah/sweep
```

## DAG Execution

Tasks can declare dependencies with `--depends-on`. The executor fans out independent tasks in parallel and fans in at dependency boundaries.

```bash
cubit todo "data pipeline"
cubit todo "train model" --depends-on 1
cubit todo "evaluate model" --depends-on 1
cubit todo "write report" --depends-on 2,3

cubit run --max-parallel 8
```

```
001 [once] data pipeline              ✓ ready
002 [once] train model                ⏳ waiting on [001]
003 [once] evaluate model             ⏳ waiting on [001]
004 [once] write report               ⏳ waiting on [002, 003]
```

## Architecture

```
~/.ark/cubit/
├── config.yaml
└── <agent>/
    ├── identity/
    │   ├── FLUCTLIGHT.md       # AI persona
    │   ├── USER.md             # User identity
    │   └── GOALS.md            # Objectives
    ├── queue/
    │   ├── 001-task-title.md   # Pending tasks
    │   ├── .doing/             # Active tasks
    │   └── done/               # Completed tasks
    ├── scratch/                # Working files
    └── memory/
        ├── MEMORY.md           # Agent's durable notes
        ├── brief.md            # Session context (auto-managed)
        ├── log.md              # Task completion log
        ├── results.tsv         # Experiment results
        └── sessions/           # Session journals
```

**Brief injection order:** FLUCTLIGHT -> USER -> GOALS -> brief.md -> active task -> scratch -> upstream outputs -> user message

## Config

`~/.ark/cubit/config.yaml`:

```yaml
root: ~/.ark/cubit
agent: noah
claude:
  model: claude-opus-4-6
  memory_model: claude-haiku-4-5-20251001
  timeout: 30m
  max_parallel: 0          # 0 = NumCPU * 4
  permission_mode: ""
  allowed_tools: []
  work_dir: ""
```

## Project Layout

```
cmd/                    # Cobra commands
  task/                 # todo, queue, do, done, requeue, log, graph
  exec/                 # prompt, run
  agent/                # identity, memory, status, brief, refresh
internal/
  config/               # Config types + Viper loading
  claude/               # Claude CLI wrapper
  queue/                # Task queue, graph, executor helpers
  brief/                # Brief injection + memory pass
  scaffold/             # Agent workspace initialization
  updater/              # Self-update from GitHub releases
main.go                 # Entry point
docs/
  v0.1/                 # M0-M1: Skeleton + init
  v0.2/                 # M2-M5: Task queue + runner
  v0.3/                 # M6-M9: Task graph + DAG executor
  v0.4/                 # M10-M12: Loop execution + memory
```

## Build

```bash
go build -o cubit .
go test ./...
```

Version injected via ldflags at build time. Release targets: `linux/amd64` + `darwin/arm64`.
