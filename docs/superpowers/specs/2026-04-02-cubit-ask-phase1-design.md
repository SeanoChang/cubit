# Cubit Ask Phase 1 — `ask write` + `ask list`

**Date:** 2026-04-02
**Status:** Design approved
**Scope:** `cubit ask write`, `cubit ask list`, `internal/ask/` package
**Source:** [Cubit Upgrade Roadmap — Ask Queue & Librarian Scaffolding](https://www.notion.so/33630938db7c8192bcc9c75df074cfcd)

---

## Overview

Add a general-purpose ask queue to cubit. Any agent can write structured JSON requests to another agent's `asks/pending/` directory. This is Phase 1: the write and list commands. Phase 2 (`ask process`) and Phase 3 (scaffold + migration) follow separately.

The ask queue is cheaper than mail (`cubit send`) — JSON instead of markdown, batched processing, minimal LLM context. Primary user is the Librarian agent, but the mechanism is not restricted.

---

## File Structure

```
cmd/ask.go              — Cobra parent + write/list subcommands
internal/ask/ask.go     — Ask struct, validation, read/write operations
```

## Filesystem Layout

```
~/.ark/agents-home/<agent>/
├── asks/
│   ├── pending/     # incoming asks (written by requester)
│   └── done/        # processed asks (responses filled in)
```

Top-level `asks/` directory, sibling to `mailbox/`. Separate because asks are JSON (not markdown) and processed differently than mail.

---

## Types (`internal/ask/ask.go`)

### Ask

```go
type Ask struct {
    ID        string       `json:"id"`
    From      string       `json:"from"`
    To        string       `json:"to"`
    Timestamp string       `json:"timestamp"`
    Action    string       `json:"action"`
    Context   AskContext   `json:"context"`
    Options   []string     `json:"options"`
    Response  *AskResponse `json:"response"`
}
```

### AskContext

```go
type AskContext struct {
    Reason         string   `json:"reason"`
    NoteIDs        []string `json:"note_ids,omitempty"`
    NoteTitles     []string `json:"note_titles,omitempty"`
    ProposedResult string   `json:"proposed_result,omitempty"`
}
```

### AskResponse

```go
type AskResponse struct {
    Decision  string `json:"decision"`
    Reason    string `json:"reason"`
    Details   any    `json:"details"`
    Timestamp string `json:"timestamp"`
    Responder string `json:"responder"`
}
```

---

## Functions (`internal/ask/ask.go`)

### `Validate(a *Ask) error`

Checks required fields:
- `to` — non-empty
- `from` — non-empty
- `action` — non-empty
- `options` — non-empty slice with at least one entry

Does NOT validate agent names (that's the caller's job, using `isValidAgentName()`).

### `GenerateID(from, action string) string`

Format: `ask-<ISO8601compact>-<from>-<action>`

Example: `ask-20260402T143000-librarian-approve-merge`

Deterministic (based on current time), sortable, human-readable.

### `Write(rootDir string, a *Ask) error`

1. Build target path: `<rootDir>/agents-home/<to>/asks/pending/<id>.json`
2. Ensure parent directory exists (`os.MkdirAll`)
3. Marshal ask to indented JSON
4. Write to temp file: `<dir>/.tmp-<id>.json`
5. `os.Rename()` temp → final (atomic)
6. Return nil on success

No copy to sender directory. The requester reads responses from `<to>/asks/done/`.

### `List(rootDir, agent string, done bool) ([]Ask, error)`

1. Build directory path: `<rootDir>/agents-home/<agent>/asks/pending/` (or `done/` if `done=true`)
2. Read directory entries, filter for `.json` files
3. Parse each file into Ask struct
4. Sort by timestamp ascending
5. Return slice

### `ListAll(rootDir string, done bool) (map[string][]Ask, error)`

1. Scan `<rootDir>/agents-home/` for agent directories
2. Call `List()` for each agent
3. Return map of agent name → asks

### `Count(rootDir, agent string) (pending int, done int, err error)`

Count `.json` files in `pending/` and `done/` without parsing.

---

## Commands (`cmd/ask.go`)

### `cubit ask` (parent)

No `RunE`. Namespace only.

### `cubit ask write`

**Input:** JSON from `--context-file <path>` or stdin. Stdin detection: `os.Stdin.Stat()` checks for pipe/redirect (mode `ModeCharDevice` unset).

**Flow:**
1. Read JSON input (file or stdin)
2. Unmarshal into `ask.Ask`
3. Inject `timestamp` (`time.Now().UTC().Format(time.RFC3339)`) if empty
4. Generate `id` via `ask.GenerateID()` if empty
5. Validate with `ask.Validate()`
6. Validate `to` and `from` with `isValidAgentName()`
7. Verify target agent directory exists
8. Call `ask.Write(cfg.Root, &a)`
9. Print JSON: `{"id": "<id>", "delivered_to": "<to>/asks/pending/"}`

**Flags:**
- `--context-file <path>` — path to JSON file (optional; reads stdin if omitted and stdin is piped)

**Errors:**
- Missing/invalid JSON → validation error
- Target agent doesn't exist → `agent directory not found: <to>`
- Invalid agent name → `invalid agent name: <name>`

### `cubit ask list`

**Flow:**
1. Determine agent(s) to query
2. Read and parse asks from filesystem
3. Output JSON

**Flags:**
- `--agent <name>` — specific agent (default: current agent from config)
- `--all` — all agents
- `--done` — show `done/` instead of `pending/`
- `--count` — counts only: `{"pending": 3, "done": 12}`

**Output formats:**

Default (single agent):
```json
[
  {"id": "ask-...", "from": "librarian", "action": "approve-merge", "timestamp": "...", ...}
]
```

With `--all`:
```json
{
  "alice": [...],
  "neo": [...]
}
```

With `--count`:
```json
{"pending": 3, "done": 12}
```

---

## ID Generation

Format: `ask-<ISO8601compact>-<from>-<action>`

- ISO8601 compact: `20260402T143000` (no separators except T)
- From: agent name as-is
- Action: as-is (free text, caller-defined)

If the caller provides an `id` in the input JSON, it is used as-is. This allows idempotent retries.

---

## Atomic Write

Same pattern as nark CAS writes:
1. Write to `<dir>/.tmp-<id>.json`
2. `os.Rename()` to `<dir>/<id>.json`

Crash between 1 and 2: orphan temp file (harmless, can be cleaned up). Crash after 2: file exists fully. No partial writes visible to readers.

---

## Registration

Add to `cmd/root.go` `init()`:
```go
rootCmd.AddCommand(askCmd)
```

---

## What This Phase Does NOT Include

- `cubit ask process` — Phase 2 (Anthropic SDK, LLM invocation)
- Scaffold updates to `cubit init` — Phase 3
- `cubit migrate-asks` — Phase 3
- TTL enforcement — keel's responsibility
- Notification — keel's responsibility
- Discord commands (`!ask`) — keel's responsibility
