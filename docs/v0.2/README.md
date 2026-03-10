# v0.2 — Linear Task Queue + Runner

**Milestones:** M2, M3, M4, M5
**Status:** Shipped

## M2: Task Queue

- `internal/queue/` — filesystem-backed task queue
- Task files: YAML frontmatter + markdown body in `queue/`
- `cubit todo`, `cubit queue`, `cubit do`, `cubit done`, `cubit requeue`, `cubit log`

## M3: Claude Runner + Prompt + Memory Pass

- `internal/claude/runner.go` — `Prompt()` wraps `claude -p` via stdin
- `RunnerOpts` for model, timeout, permission mode, allowed tools
- `internal/brief/` — session brief injection (FLUCTLIGHT -> USER -> GOALS -> brief.md -> task -> scratch)
- `RunMemoryPass()` — cheap LLM call to rewrite brief.md after sessions
- `cubit prompt` command with brief injection

## M4: `cubit run`

- Linear task executor: pop -> prompt -> done -> next
- `--once`, `--cooldown` flags

## M5: Status, Refresh, `--no-memory`

- `cubit status` — agent name, active task, queue depth, brief token size
- `cubit refresh` — rewrite brief.md from scratch using recent journals + log
- `--no-memory` flag on `prompt` and `run`
