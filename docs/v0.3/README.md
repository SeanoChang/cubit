# v0.3 — Task Graph + DAG Executor

**Milestones:** M6, M7, M8, M9
**Status:** Shipped

## M6: Task Frontmatter

- Added `mode`, `depends_on`, `program`, `goal`, `max_iterations`, `branch`, `model` fields to Task struct
- `CreateOptions` struct for `Queue.Create()`

## M7: `cubit todo` Flags

- Wired all frontmatter fields as `cubit todo` flags
- Cycle validation via `ValidateDependencies()` before task creation

## M8: `cubit graph`

- `internal/queue/graph.go` — `BuildGraph()`, `DetectCycle()` (DFS coloring)
- Done task persistence in `queue/done/`
- `cubit graph` with `--status`, `--mode`, `--ascii` filters

## M9: Concurrent DAG Executor

- Replaced linear `cubit run` with fan-out/fan-in DAG executor
- Event-driven scheduler: main goroutine dispatches, workers are pure functions
- Semaphore-gated concurrency (`--max-parallel`)
- `.doing/` directory for multiple active tasks
- Upstream output injection for fan-in nodes
- Deadlock detection, graceful shutdown on SIGINT
- `cubit do --all` to pop all ready nodes

## Identity CLI

- `cubit identity list|show|set` — manage FLUCTLIGHT.md, USER.md, GOALS.md
- Path traversal protection
