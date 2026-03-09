# M9 — Concurrent DAG Executor (Fan-out / Fan-in)

**Date:** 2026-03-09
**Status:** Approved
**Milestone:** M9 (follows M0–M8, all shipped)

## Goal

Replace the linear `cubit run` executor with a concurrent DAG executor. Multiple ready nodes execute in parallel, fan-in nodes wait for all upstream deps, deadlock is detected automatically.

## Approach: Event-driven with Semaphore

Each completed task triggers an immediate re-scan for newly ready nodes. No batch boundaries — a fast task unblocks its dependents immediately.

```
for {
    ready := readyNodes(graph)

    for _, task := range ready {
        sem.Acquire()
        running++
        go func(t *Task) {
            defer sem.Release()
            result := executeWithRetry(t, cfg, 3)
            doneCh <- result
        }(task)
    }

    if running == 0 {
        if graphComplete(graph) { return nil }
        return errDeadlock(graph)
    }

    result := <-doneCh
    running--
    markDone(graph, result)
    runMemoryPass(result)
}
```

**No mutex.** Main goroutine is the only reader/writer of graph state. Workers are pure functions: take task, build brief, call claude, return result.

## Filesystem Changes

### `.doing` becomes a directory

```
queue/
├── 001-sweep.md            # pending
├── .doing/                 # active tasks (was single file)
│   ├── 003-report.md       # running
│   └── 004-review.md       # running
└── done/
    └── 000-setup.md        # completed
```

- `Pop()` moves a file into `.doing/` (keeps original filename)
- `PopMulti(ids []int)` pops multiple ready nodes at once
- `Active()` returns `[]*Task` (was single `*Task`)
- `Complete(id, summary)` takes explicit task ID
- Migration: old `.doing` file auto-moved into `.doing/` directory

### Output capture

- Each completed task's output → `scratch/<NNN>-output.md`
- Fan-in nodes get paths injected into brief (model decides read order)

## Concurrent Executor

### Workers

Pure functions — no shared state:
1. Build brief + inject upstream output paths for fan-in nodes
2. Call `claude.Prompt(brief, task.Model)` (per-task model override)
3. Return result on `doneCh`

### Retry

- Up to 3 attempts per task on failure
- After 3 failures: mark done with failure note in log.md, write empty output file
- Other parallel tasks continue unaffected

### Graceful shutdown

- SIGINT sets cancel flag
- No new tasks dispatched
- Running tasks finish current work
- Then exit

### Deadlock detection

Falls out naturally from the loop: `running == 0 && !graphComplete(graph)` = deadlock. Print which nodes are stuck and their unmet deps.

## Config & Flags

```yaml
claude:
  model: "claude-opus-4-6"       # default model
  timeout: 30m
  max_parallel: 0                 # 0 = runtime.NumCPU() * 4
```

Per-task model override via frontmatter:
```yaml
---
id: 003
model: "claude-sonnet-4-6"
---
```

Valid models: `claude-opus-4-6`, `claude-sonnet-4-6`, `claude-haiku-4-5`.

Flags:
```bash
cubit run --max-parallel 10      # override config
cubit run --once                 # do one task, stop
cubit run --cooldown 2m          # wait between tasks
cubit do --all                   # pop all ready nodes
```

## Brief Injection for Fan-in Nodes

When a task has `depends_on` with completed upstream outputs:

```
1. FLUCTLIGHT.md
2. USER.md
3. GOALS.md
4. brief.md
5. Active task (.doing/<NNN>-slug.md)
6. scratch/plan.md (if exists)
7. ## Upstream Results              ← NEW
   - scratch/001-output.md
   - scratch/002-output.md
8. User message
```

`Build()` gains optional `upstreamIDs []int`. For each ID, checks if `scratch/<NNN>-output.md` exists and lists the path.

## `cubit do` Changes

```bash
cubit do          # pop one ready node into .doing/
cubit do --all    # pop all ready nodes into .doing/
```

- Only pops nodes whose deps are all satisfied
- Rejects if node has unmet deps

## Files to Modify

- `internal/queue/queue.go` — `.doing` dir, multi-task Pop/Active/Complete, PopAll
- `internal/queue/graph.go` — add `ReadyNodes()`, `GraphComplete()`
- `internal/brief/brief.go` — upstream output path injection
- `internal/config/config.go` — add `MaxParallel` field
- `cmd/run.go` — replace linear loop with concurrent executor
- `cmd/do.go` — `--all` flag, only pop ready nodes
- `cmd/root.go` — register `--max-parallel`, `--all` flags

## Files to Create

- `internal/queue/executor.go` — concurrent run loop, executeWithRetry, output capture

## Test Coverage

- `ReadyNodes()` — various graph states
- `GraphComplete()` — partial vs full resolution
- `.doing/` directory lifecycle — pop, complete, requeue with multiple active
- Executor — fan-out/fan-in ordering, deadlock detection, retry exhaustion
- Migration — old `.doing` file → `.doing/` directory
