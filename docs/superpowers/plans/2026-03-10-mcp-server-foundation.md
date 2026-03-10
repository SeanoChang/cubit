# MCP Server Foundation (v0.5 Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose cubit operations as MCP tools so Claude Code can call them natively — no subprocess nesting, no manual prompting.

**Architecture:** `cubit mcp` starts a stdio JSON-RPC 2.0 server that reuses `internal/queue` and `internal/config` as backend. Each MCP tool maps 1:1 to an existing queue operation. The server registers in `cmd/root.go` via the existing `Register()` pattern.

**Tech Stack:** Go 1.26, Cobra, JSON-RPC 2.0 (custom — no external MCP lib), existing `internal/queue` + `internal/brief` packages.

**Spec:** Notion — "Cubit v0.5 Update" → "Phase 1: MCP Server Foundation"

---

## File Structure

```
cmd/mcp/
  ├── register.go          # Register() wires `cubit mcp` into root, follows existing pattern
  └── server.go            # Cobra command — starts MCP server on stdio

internal/mcp/
  ├── protocol.go          # JSON-RPC 2.0 types: Request, Response, Error, Tool, ToolResult
  ├── server.go            # Read loop, method dispatch (initialize, tools/list, tools/call)
  ├── tools.go             # Tool registry map + dispatch helper
  ├── tool_queue.go        # cubit_queue: list tasks by filter
  ├── tool_todo.go         # cubit_todo: create task with deps/mode/goal
  ├── tool_do.go           # cubit_do: pop next ready task
  ├── tool_done.go         # cubit_done: complete task by ID
  ├── tool_requeue.go      # cubit_requeue: return task to pending
  ├── tool_log.go          # cubit_log: append observation
  ├── tool_graph.go        # cubit_graph: render DAG (ASCII or Mermaid)
  └── tool_status.go       # cubit_status: queue health + brief token size

internal/mcp/mcp_test.go   # All MCP tests in one file (protocol + tools)
```

**Modified files:**
- `cmd/root.go` — add `mcp.Register(rootCmd, getCfg, getQ)` import + call

---

## Chunk 1: MCP Server Skeleton (Issues #1–#2)

### Task 1: Protocol types

**Files:**
- Create: `internal/mcp/protocol.go`

- [ ] **Step 1: Create protocol types file**

```go
// Package mcp implements a stdio MCP server for cubit.
package mcp

import "encoding/json"

// JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // number or string; nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP tool definition returned by tools/list.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// MCP tool result returned by tools/call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a text content block in MCP tool results.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// textResult is a convenience for returning a single text block.
func textResult(s string) *ToolResult {
	return &ToolResult{Content: []ContentBlock{{Type: "text", Text: s}}}
}

// errorResult is a convenience for returning a tool-level error.
func errorResult(s string) *ToolResult {
	return &ToolResult{Content: []ContentBlock{{Type: "text", Text: s}}, IsError: true}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go build ./internal/mcp/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/protocol.go
git commit -m "feat(mcp): add JSON-RPC 2.0 protocol types"
```

---

### Task 2: Server read loop + initialize handler

**Files:**
- Create: `internal/mcp/server.go`

- [ ] **Step 1: Write the server**

The server reads newline-delimited JSON from an `io.Reader`, dispatches methods, and writes responses to an `io.Writer`. Using Reader/Writer (not stdin/stdout directly) makes it testable.

```go
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
)

// Server is a stdio MCP server.
type Server struct {
	cfg   *config.Config
	q     *queue.Queue
	tools *Registry
}

// NewServer creates an MCP server backed by the given config and queue.
func NewServer(cfg *config.Config, q *queue.Queue) *Server {
	s := &Server{cfg: cfg, q: q}
	s.tools = NewRegistry(q, cfg)
	return s
}

// Run reads JSON-RPC requests from r and writes responses to w.
// Blocks until r is closed or an unrecoverable error occurs.
func (s *Server) Run(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp := Response{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32700, Message: "parse error"},
			}
			s.writeResponse(w, resp)
			continue
		}

		resp := s.handle(req)
		if resp != nil {
			s.writeResponse(w, *resp)
		}
	}
	return scanner.Err()
}

func (s *Server) handle(req Request) *Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		return nil // notification, no response
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *Server) handleInitialize(req Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "cubit",
				"version": "0.5.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": s.tools.Definitions(),
		},
	}
}

func (s *Server) handleToolsCall(req Request) *Response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "invalid params"},
		}
	}

	result := s.tools.Call(params.Name, params.Arguments)
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) writeResponse(w io.Writer, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("mcp: marshal error: %v", err)
		return
	}
	fmt.Fprintf(w, "%s\n", data)
}
```

- [ ] **Step 2: Verify it compiles** (will fail — needs Registry, built next)

This is expected. Move to Task 3 to create the registry, then come back.

---

### Task 3: Tool registry + cubit_queue

**Files:**
- Create: `internal/mcp/tools.go`
- Create: `internal/mcp/tool_queue.go`

- [ ] **Step 1: Write the tool registry**

```go
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
)

// ToolHandler processes a tool call and returns a result.
type ToolHandler func(args json.RawMessage) *ToolResult

// Registry holds tool definitions and handlers.
type Registry struct {
	q    *queue.Queue
	cfg  *config.Config
	defs []ToolDef
	handlers map[string]ToolHandler
}

// NewRegistry creates a registry with all cubit MCP tools.
func NewRegistry(q *queue.Queue, cfg *config.Config) *Registry {
	r := &Registry{
		q:        q,
		cfg:      cfg,
		handlers: make(map[string]ToolHandler),
	}
	r.registerQueue()
	return r
}

// register adds a tool definition and handler.
func (r *Registry) register(def ToolDef, handler ToolHandler) {
	r.defs = append(r.defs, def)
	r.handlers[def.Name] = handler
}

// Definitions returns all tool definitions for tools/list.
func (r *Registry) Definitions() []ToolDef {
	return r.defs
}

// Call dispatches a tool call by name.
func (r *Registry) Call(name string, args json.RawMessage) *ToolResult {
	handler, ok := r.handlers[name]
	if !ok {
		return errorResult(fmt.Sprintf("unknown tool: %s", name))
	}
	return handler(args)
}
```

- [ ] **Step 2: Write cubit_queue tool**

```go
package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (r *Registry) registerQueue() {
	r.register(ToolDef{
		Name:        "cubit_queue",
		Description: "List tasks in the queue. Returns pending, active, and/or done tasks.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filter": map[string]any{
					"type":        "string",
					"enum":        []string{"pending", "active", "done", "all"},
					"description": "Filter by status. Default: all.",
				},
			},
		},
	}, r.handleQueue)
}

func (r *Registry) handleQueue(args json.RawMessage) *ToolResult {
	var params struct {
		Filter string `json:"filter"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &params)
	}
	if params.Filter == "" {
		params.Filter = "all"
	}

	var sections []string

	if params.Filter == "pending" || params.Filter == "all" {
		tasks, err := r.q.List()
		if err != nil {
			return errorResult(fmt.Sprintf("listing pending: %v", err))
		}
		var lines []string
		for _, t := range tasks {
			lines = append(lines, fmt.Sprintf("  %03d [%s] %s", t.ID, t.Mode, t.Title))
		}
		if len(lines) == 0 {
			sections = append(sections, "Pending: (none)")
		} else {
			sections = append(sections, "Pending:\n"+strings.Join(lines, "\n"))
		}
	}

	if params.Filter == "active" || params.Filter == "all" {
		tasks, err := r.q.Active()
		if err != nil {
			return errorResult(fmt.Sprintf("listing active: %v", err))
		}
		var lines []string
		for _, t := range tasks {
			lines = append(lines, fmt.Sprintf("  %03d [%s] %s", t.ID, t.Mode, t.Title))
		}
		if len(lines) == 0 {
			sections = append(sections, "Active: (none)")
		} else {
			sections = append(sections, "Active:\n"+strings.Join(lines, "\n"))
		}
	}

	if params.Filter == "done" || params.Filter == "all" {
		tasks, err := r.q.ListDone()
		if err != nil {
			return errorResult(fmt.Sprintf("listing done: %v", err))
		}
		var lines []string
		for _, t := range tasks {
			lines = append(lines, fmt.Sprintf("  %03d [%s] %s", t.ID, t.Mode, t.Title))
		}
		if len(lines) == 0 {
			sections = append(sections, "Done: (none)")
		} else {
			sections = append(sections, "Done:\n"+strings.Join(lines, "\n"))
		}
	}

	return textResult(strings.Join(sections, "\n\n"))
}
```

- [ ] **Step 3: Verify everything compiles**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go build ./internal/mcp/`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server.go internal/mcp/tools.go internal/mcp/tool_queue.go
git commit -m "feat(mcp): add server read loop, tool registry, and cubit_queue tool"
```

---

### Task 4: Cobra command + root wiring

**Files:**
- Create: `cmd/mcp/register.go`
- Create: `cmd/mcp/server.go`
- Modify: `cmd/root.go:7-12` (add import), `cmd/root.go:61-63` (add Register call)

- [ ] **Step 1: Write cmd/mcp/register.go**

Follow the exact pattern from `cmd/task/register.go`:

```go
package mcp

import (
	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
)

var (
	getCfg func() *config.Config
	getQ   func() *queue.Queue
)

// Register adds the cubit mcp command to root.
func Register(root *cobra.Command, cfgFn func() *config.Config, qFn func() *queue.Queue) {
	getCfg = cfgFn
	getQ = qFn
	root.AddCommand(mcpCmd)
}
```

- [ ] **Step 2: Write cmd/mcp/server.go**

```go
package mcp

import (
	"os"

	internalmcp "github.com/SeanoChang/cubit/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server on stdio",
	Long:  "Starts a Model Context Protocol server over stdin/stdout for Claude Code integration.",
	RunE: func(cmd *cobra.Command, args []string) error {
		srv := internalmcp.NewServer(getCfg(), getQ())
		return srv.Run(os.Stdin, os.Stdout)
	},
}
```

- [ ] **Step 3: Wire into cmd/root.go**

Add import `"github.com/SeanoChang/cubit/cmd/mcp"` (alias as `mcpcmd` to avoid collision with the internal package).

Add `mcpcmd.Register(rootCmd, getCfg, getQ)` after the agent.Register line.

In `cmd/root.go`:
- Add to imports: `mcpcmd "github.com/SeanoChang/cubit/cmd/mcp"`
- Add after line 63 (`agent.Register(...)`): `mcpcmd.Register(rootCmd, getCfg, getQ)`

- [ ] **Step 4: Verify full build**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go build -o cubit . && ./cubit mcp --help`
Expected: prints help text for `cubit mcp`

- [ ] **Step 5: Commit**

```bash
git add cmd/mcp/register.go cmd/mcp/server.go cmd/root.go
git commit -m "feat(mcp): add cubit mcp cobra command and wire into root"
```

---

### Task 5: Tests for protocol + server + cubit_queue

**Files:**
- Create: `internal/mcp/mcp_test.go`

- [ ] **Step 1: Write test helpers and protocol tests**

```go
package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
)

// setupTest creates a temp agent dir and returns a fresh Server.
func setupTest(t *testing.T) (*Server, string) {
	t.Helper()
	// Reset queue singleton
	queue.ResetForTest()

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "queue"), 0o755)
	os.MkdirAll(filepath.Join(dir, "queue", ".doing"), 0o755)
	os.MkdirAll(filepath.Join(dir, "queue", "done"), 0o755)
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.MkdirAll(filepath.Join(dir, "scratch"), 0o755)
	os.MkdirAll(filepath.Join(dir, "identity"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "log.md"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "memory", "brief.md"), []byte("test brief"), 0o644)
	os.WriteFile(filepath.Join(dir, "memory", "MEMORY.md"), []byte(""), 0o644)

	q := queue.GetQueue(dir)
	cfg := config.Default("test")
	cfg.Root = filepath.Dir(dir)
	cfg.Agent = filepath.Base(dir)

	srv := NewServer(cfg, q)
	return srv, dir
}

// roundTrip sends a JSON-RPC request and returns the parsed response.
func roundTrip(t *testing.T, srv *Server, method string, params any) Response {
	t.Helper()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
	}
	if params != nil {
		p, _ := json.Marshal(params)
		req["params"] = json.RawMessage(p)
	}

	line, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	line = append(line, '\n')

	var out bytes.Buffer
	srv.Run(bytes.NewReader(line), &out)

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nraw: %s", err, out.String())
	}
	return resp
}

// callTool is a convenience for tools/call round trips.
func callTool(t *testing.T, srv *Server, name string, args any) ToolResult {
	t.Helper()
	resp := roundTrip(t, srv, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if resp.Error != nil {
		t.Fatalf("tools/call error: %s", resp.Error.Message)
	}
	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	json.Unmarshal(data, &result)
	return result
}

func TestInitialize(t *testing.T) {
	srv, _ := setupTest(t)
	resp := roundTrip(t, srv, "initialize", nil)

	if resp.Error != nil {
		t.Fatalf("initialize error: %s", resp.Error.Message)
	}
	data, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(data), "cubit") {
		t.Errorf("initialize result missing server name: %s", data)
	}
}

func TestToolsList(t *testing.T) {
	srv, _ := setupTest(t)
	resp := roundTrip(t, srv, "tools/list", nil)

	if resp.Error != nil {
		t.Fatalf("tools/list error: %s", resp.Error.Message)
	}
	data, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(data), "cubit_queue") {
		t.Errorf("tools/list missing cubit_queue: %s", data)
	}
}

func TestUnknownMethod(t *testing.T) {
	srv, _ := setupTest(t)
	resp := roundTrip(t, srv, "bogus/method", nil)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
}

func TestQueueEmpty(t *testing.T) {
	srv, _ := setupTest(t)
	result := callTool(t, srv, "cubit_queue", nil)

	text := result.Content[0].Text
	if !strings.Contains(text, "Pending: (none)") {
		t.Errorf("expected empty pending, got: %s", text)
	}
}

func TestQueueWithTasks(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("build MCP server", queue.CreateOptions{})
	srv.q.Create("add tests", queue.CreateOptions{})

	result := callTool(t, srv, "cubit_queue", map[string]string{"filter": "pending"})
	text := result.Content[0].Text
	if !strings.Contains(text, "001") || !strings.Contains(text, "build MCP server") {
		t.Errorf("missing task in output: %s", text)
	}
}

func TestUnknownTool(t *testing.T) {
	srv, _ := setupTest(t)
	result := callTool(t, srv, "cubit_bogus", nil)

	if !result.IsError {
		t.Error("expected IsError for unknown tool")
	}
}
```

- [ ] **Step 2: Expose queue singleton reset for tests**

Add to `internal/queue/queue.go` after the `GetQueue` function:

```go
// ResetForTest clears the singleton. Test-only.
func ResetForTest() {
	instance = nil
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -v -count=1`
Expected: all tests PASS

- [ ] **Step 4: Run full test suite to ensure no regressions**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./... -count=1`
Expected: all tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/mcp_test.go internal/queue/queue.go
git commit -m "test(mcp): add tests for protocol, server, and cubit_queue tool"
```

---

## Chunk 2: Task CRUD Tools (Issue #3)

### Task 6: cubit_todo tool

**Files:**
- Create: `internal/mcp/tool_todo.go`
- Modify: `internal/mcp/tools.go` (add `r.registerTodo()` in `NewRegistry`)

- [ ] **Step 1: Write the failing test**

Add to `internal/mcp/mcp_test.go`:

```go
func TestTodoCreateTask(t *testing.T) {
	srv, _ := setupTest(t)
	result := callTool(t, srv, "cubit_todo", map[string]any{
		"description": "implement MCP server",
	})
	if result.IsError {
		t.Fatalf("cubit_todo error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "001") {
		t.Errorf("expected task ID 001 in result: %s", text)
	}
}

func TestTodoWithAllOptions(t *testing.T) {
	srv, _ := setupTest(t)

	// Create dependency first
	srv.q.Create("prerequisite", queue.CreateOptions{})

	result := callTool(t, srv, "cubit_todo", map[string]any{
		"description":    "sweep architecture",
		"context":        "baseline val_bpb: 0.997",
		"mode":           "loop",
		"depends_on":     []int{1},
		"program":        "sweep.md",
		"goal":           "val_bpb < 0.95",
		"max_iterations": 50,
		"branch":         "noah/sweep",
		"model":          "claude-sonnet-4-6",
	})
	if result.IsError {
		t.Fatalf("cubit_todo error: %s", result.Content[0].Text)
	}

	// Verify the task was created correctly
	tasks, _ := srv.q.List()
	found := false
	for _, task := range tasks {
		if task.Title == "sweep architecture" {
			found = true
			if task.Mode != "loop" {
				t.Errorf("mode = %q, want loop", task.Mode)
			}
			if len(task.DependsOn) != 1 || task.DependsOn[0] != 1 {
				t.Errorf("depends_on = %v, want [1]", task.DependsOn)
			}
			if task.Program != "sweep.md" {
				t.Errorf("program = %q, want sweep.md", task.Program)
			}
			if task.Goal != "val_bpb < 0.95" {
				t.Errorf("goal = %q", task.Goal)
			}
			if task.MaxIterations != 50 {
				t.Errorf("max_iterations = %d, want 50", task.MaxIterations)
			}
		}
	}
	if !found {
		t.Error("task 'sweep architecture' not found in queue")
	}
}

func TestTodoCycleDetection(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("A", queue.CreateOptions{DependsOn: []int{2}})

	result := callTool(t, srv, "cubit_todo", map[string]any{
		"description": "B",
		"depends_on":  []int{1},
	})
	if !result.IsError {
		t.Error("expected error for cycle")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -run TestTodo -v -count=1`
Expected: FAIL — cubit_todo not registered

- [ ] **Step 3: Implement cubit_todo**

```go
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/SeanoChang/cubit/internal/queue"
)

func (r *Registry) registerTodo() {
	r.register(ToolDef{
		Name: "cubit_todo",
		Description: `Create a new task in the queue. When creating a multi-step plan, always include a terminal summary node that depends on all leaf tasks. Use fan-out/fan-in patterns for parallel work.`,
		InputSchema: map[string]any{
			"type": "object",
			"required": []string{"description"},
			"properties": map[string]any{
				"description": map[string]any{
					"type":        "string",
					"description": "Task description (becomes the title)",
				},
				"context": map[string]any{
					"type":        "string",
					"description": "Additional context appended to the task body",
				},
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"once", "loop"},
					"description": "Execution mode. Default: once",
				},
				"depends_on": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "integer"},
					"description": "Task IDs this task depends on",
				},
				"program": map[string]any{
					"type":        "string",
					"description": "Program file re-injected each loop iteration",
				},
				"goal": map[string]any{
					"type":        "string",
					"description": "Exit condition for loop mode",
				},
				"max_iterations": map[string]any{
					"type":        "integer",
					"description": "Max loop iterations (0 = unlimited)",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Claude model override for this task",
				},
				"branch": map[string]any{
					"type":        "string",
					"description": "Git branch convention for this task",
				},
			},
		},
	}, r.handleTodo)
}

func (r *Registry) handleTodo(args json.RawMessage) *ToolResult {
	var params struct {
		Description   string `json:"description"`
		Context       string `json:"context"`
		Mode          string `json:"mode"`
		DependsOn     []int  `json:"depends_on"`
		Program       string `json:"program"`
		Goal          string `json:"goal"`
		MaxIterations int    `json:"max_iterations"`
		Model         string `json:"model"`
		Branch        string `json:"branch"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult(fmt.Sprintf("invalid arguments: %v", err))
	}
	if params.Description == "" {
		return errorResult("description is required")
	}

	// Validate dependencies before creating
	if len(params.DependsOn) > 0 {
		nextID := r.q.NextID()
		if err := r.q.ValidateDependencies(nextID, params.DependsOn); err != nil {
			return errorResult(fmt.Sprintf("dependency error: %v", err))
		}
	}

	task, err := r.q.Create(params.Description, queue.CreateOptions{
		Context:       params.Context,
		Mode:          params.Mode,
		Model:         params.Model,
		DependsOn:     params.DependsOn,
		Program:       params.Program,
		Goal:          params.Goal,
		MaxIterations: params.MaxIterations,
		Branch:        params.Branch,
	})
	if err != nil {
		return errorResult(fmt.Sprintf("create task: %v", err))
	}

	return textResult(fmt.Sprintf("Created task %03d: %s", task.ID, task.Title))
}
```

- [ ] **Step 4: Register in NewRegistry**

In `internal/mcp/tools.go`, add `r.registerTodo()` after `r.registerQueue()` in `NewRegistry()`.

- [ ] **Step 5: Run tests**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -run TestTodo -v -count=1`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/tool_todo.go internal/mcp/tools.go internal/mcp/mcp_test.go
git commit -m "feat(mcp): add cubit_todo tool with dependency validation"
```

---

### Task 7: cubit_do tool

**Files:**
- Create: `internal/mcp/tool_do.go`
- Modify: `internal/mcp/tools.go` (add `r.registerDo()`)

- [ ] **Step 1: Write the failing test**

Add to `internal/mcp/mcp_test.go`:

```go
func TestDoPopReady(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("task A", queue.CreateOptions{})
	srv.q.Create("task B", queue.CreateOptions{DependsOn: []int{1}})

	result := callTool(t, srv, "cubit_do", nil)
	if result.IsError {
		t.Fatalf("cubit_do error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "001") || !strings.Contains(text, "task A") {
		t.Errorf("expected task A popped: %s", text)
	}

	// Task B should still be pending (blocked)
	active, _ := srv.q.Active()
	if len(active) != 1 || active[0].ID != 1 {
		t.Errorf("expected task 1 active, got %v", active)
	}
}

func TestDoNoReady(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("blocked", queue.CreateOptions{DependsOn: []int{99}})

	result := callTool(t, srv, "cubit_do", nil)
	if !result.IsError {
		t.Error("expected error when no tasks ready")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -run TestDo -v -count=1`

- [ ] **Step 3: Implement cubit_do**

```go
package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (r *Registry) registerDo() {
	r.register(ToolDef{
		Name:        "cubit_do",
		Description: "Pop and return the next ready task (dependencies satisfied). Use all=true to pop all ready tasks at once for parallel execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"all": map[string]any{
					"type":        "boolean",
					"description": "Pop all ready tasks at once. Default: false",
				},
			},
		},
	}, r.handleDo)
}

func (r *Registry) handleDo(args json.RawMessage) *ToolResult {
	var params struct {
		All bool `json:"all"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &params)
	}

	if params.All {
		tasks, err := r.q.PopAllReady()
		if err != nil {
			return errorResult(fmt.Sprintf("pop all ready: %v", err))
		}
		if len(tasks) == 0 {
			return errorResult("no ready tasks")
		}
		var lines []string
		for _, t := range tasks {
			lines = append(lines, fmt.Sprintf("  %03d [%s] %s", t.ID, t.Mode, t.Title))
		}
		return textResult(fmt.Sprintf("Popped %d tasks:\n%s", len(tasks), strings.Join(lines, "\n")))
	}

	task, err := r.q.PopReady()
	if err != nil {
		return errorResult(fmt.Sprintf("pop ready: %v", err))
	}
	return textResult(fmt.Sprintf("Popped task %03d [%s]: %s", task.ID, task.Mode, task.Title))
}
```

- [ ] **Step 4: Register in NewRegistry**

Add `r.registerDo()` in `NewRegistry()`.

- [ ] **Step 5: Run tests**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -run TestDo -v -count=1`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/tool_do.go internal/mcp/tools.go internal/mcp/mcp_test.go
git commit -m "feat(mcp): add cubit_do tool"
```

---

### Task 8: cubit_done tool

**Files:**
- Create: `internal/mcp/tool_done.go`
- Modify: `internal/mcp/tools.go` (add `r.registerDone()`)

- [ ] **Step 1: Write the failing test**

Add to `internal/mcp/mcp_test.go`:

```go
func TestDoneCompleteTask(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("task A", queue.CreateOptions{})
	srv.q.PopReady()

	result := callTool(t, srv, "cubit_done", map[string]any{
		"id":      1,
		"summary": "finished implementing",
	})
	if result.IsError {
		t.Fatalf("cubit_done error: %s", result.Content[0].Text)
	}

	done, _ := srv.q.ListDone()
	if len(done) != 1 || done[0].ID != 1 {
		t.Errorf("expected task 1 done, got %v", done)
	}
}

func TestDoneInvalidID(t *testing.T) {
	srv, _ := setupTest(t)
	result := callTool(t, srv, "cubit_done", map[string]any{"id": 999})
	if !result.IsError {
		t.Error("expected error for invalid ID")
	}
}
```

- [ ] **Step 2: Implement cubit_done**

```go
package mcp

import (
	"encoding/json"
	"fmt"
)

func (r *Registry) registerDone() {
	r.register(ToolDef{
		Name:        "cubit_done",
		Description: "Mark an active task as complete. Appends a log entry with the summary.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "integer",
					"description": "Task ID to complete",
				},
				"summary": map[string]any{
					"type":        "string",
					"description": "Completion summary (logged to memory/log.md)",
				},
			},
		},
	}, r.handleDone)
}

func (r *Registry) handleDone(args json.RawMessage) *ToolResult {
	var params struct {
		ID      int    `json:"id"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult(fmt.Sprintf("invalid arguments: %v", err))
	}
	if params.ID == 0 {
		return errorResult("id is required")
	}

	if err := r.q.CompleteByID(params.ID, params.Summary); err != nil {
		return errorResult(fmt.Sprintf("complete task: %v", err))
	}
	return textResult(fmt.Sprintf("Completed task %03d", params.ID))
}
```

- [ ] **Step 3: Register and run tests**

Add `r.registerDone()` in `NewRegistry()`.

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -run TestDone -v -count=1`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tool_done.go internal/mcp/tools.go internal/mcp/mcp_test.go
git commit -m "feat(mcp): add cubit_done tool"
```

---

### Task 9: cubit_requeue tool

**Files:**
- Create: `internal/mcp/tool_requeue.go`
- Modify: `internal/mcp/tools.go` (add `r.registerRequeue()`)

- [ ] **Step 1: Write the failing test**

Add to `internal/mcp/mcp_test.go`:

```go
func TestRequeueTask(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("task A", queue.CreateOptions{})
	srv.q.PopReady()

	result := callTool(t, srv, "cubit_requeue", map[string]any{"id": 1})
	if result.IsError {
		t.Fatalf("cubit_requeue error: %s", result.Content[0].Text)
	}

	pending, _ := srv.q.List()
	if len(pending) != 1 || pending[0].ID != 1 {
		t.Errorf("expected task 1 back in pending, got %v", pending)
	}

	active, _ := srv.q.Active()
	if len(active) != 0 {
		t.Errorf("expected no active tasks, got %d", len(active))
	}
}
```

- [ ] **Step 2: Implement cubit_requeue**

```go
package mcp

import (
	"encoding/json"
	"fmt"
)

func (r *Registry) registerRequeue() {
	r.register(ToolDef{
		Name:        "cubit_requeue",
		Description: "Return an active task to pending status. Use when a task needs to be retried or was popped by mistake.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "integer",
					"description": "Task ID to requeue",
				},
			},
		},
	}, r.handleRequeue)
}

func (r *Registry) handleRequeue(args json.RawMessage) *ToolResult {
	var params struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult(fmt.Sprintf("invalid arguments: %v", err))
	}
	if params.ID == 0 {
		return errorResult("id is required")
	}

	if err := r.q.RequeueByID(params.ID); err != nil {
		return errorResult(fmt.Sprintf("requeue task: %v", err))
	}
	return textResult(fmt.Sprintf("Requeued task %03d", params.ID))
}
```

- [ ] **Step 3: Register and run tests**

Add `r.registerRequeue()` in `NewRegistry()`.

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -run TestRequeue -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tool_requeue.go internal/mcp/tools.go internal/mcp/mcp_test.go
git commit -m "feat(mcp): add cubit_requeue tool"
```

---

### Task 10: cubit_log tool

**Files:**
- Create: `internal/mcp/tool_log.go`
- Modify: `internal/mcp/tools.go` (add `r.registerLog()`)

- [ ] **Step 1: Write the failing test**

Add to `internal/mcp/mcp_test.go`:

```go
func TestLogObservation(t *testing.T) {
	srv, dir := setupTest(t)
	result := callTool(t, srv, "cubit_log", map[string]any{
		"note": "discovered important pattern",
	})
	if result.IsError {
		t.Fatalf("cubit_log error: %s", result.Content[0].Text)
	}

	logData, _ := os.ReadFile(filepath.Join(dir, "memory", "log.md"))
	if !strings.Contains(string(logData), "discovered important pattern") {
		t.Errorf("log.md missing observation: %s", logData)
	}
}
```

- [ ] **Step 2: Implement cubit_log**

```go
package mcp

import (
	"encoding/json"
	"fmt"
)

func (r *Registry) registerLog() {
	r.register(ToolDef{
		Name:        "cubit_log",
		Description: "Append a free-form observation to memory/log.md. Use for insights, blockers, decisions, or anything worth remembering.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"note"},
			"properties": map[string]any{
				"note": map[string]any{
					"type":        "string",
					"description": "The observation to log",
				},
			},
		},
	}, r.handleLog)
}

func (r *Registry) handleLog(args json.RawMessage) *ToolResult {
	var params struct {
		Note string `json:"note"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult(fmt.Sprintf("invalid arguments: %v", err))
	}
	if params.Note == "" {
		return errorResult("note is required")
	}

	if err := r.q.Log(params.Note); err != nil {
		return errorResult(fmt.Sprintf("log: %v", err))
	}
	return textResult("Logged observation")
}
```

- [ ] **Step 3: Register and run tests**

Add `r.registerLog()` in `NewRegistry()`.

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -run TestLog -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tool_log.go internal/mcp/tools.go internal/mcp/mcp_test.go
git commit -m "feat(mcp): add cubit_log tool"
```

---

## Chunk 3: Graph + Status Tools (Issue #4)

### Task 11: cubit_graph tool

**Files:**
- Create: `internal/mcp/tool_graph.go`
- Modify: `internal/mcp/tools.go` (add `r.registerGraph()`)

- [ ] **Step 1: Write the failing test**

Add to `internal/mcp/mcp_test.go`:

```go
func TestGraphFullDAG(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("root A", queue.CreateOptions{})
	srv.q.Create("root B", queue.CreateOptions{})
	srv.q.Create("merge", queue.CreateOptions{DependsOn: []int{1, 2}})

	result := callTool(t, srv, "cubit_graph", nil)
	if result.IsError {
		t.Fatalf("cubit_graph error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	// Should contain Mermaid graph by default
	if !strings.Contains(text, "graph TD") {
		t.Errorf("expected Mermaid graph, got: %s", text)
	}
	if !strings.Contains(text, "001") || !strings.Contains(text, "003") {
		t.Errorf("missing task IDs in graph: %s", text)
	}
}

func TestGraphASCIISubgraph(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("root", queue.CreateOptions{})
	srv.q.Create("child", queue.CreateOptions{DependsOn: []int{1}})

	result := callTool(t, srv, "cubit_graph", map[string]any{
		"id":    2,
		"ascii": true,
	})
	if result.IsError {
		t.Fatalf("cubit_graph error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Depends on") {
		t.Errorf("expected ASCII tree with deps, got: %s", text)
	}
}
```

- [ ] **Step 2: Implement cubit_graph**

```go
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/SeanoChang/cubit/internal/queue"
)

func (r *Registry) registerGraph() {
	r.register(ToolDef{
		Name:        "cubit_graph",
		Description: "Visualize the task DAG. Returns Mermaid by default, or ASCII tree for a specific task subgraph. Use to inspect task dependencies and plan structure.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "integer",
					"description": "Task ID to show subgraph for. Omit for full graph.",
				},
				"ascii": map[string]any{
					"type":        "boolean",
					"description": "Render as ASCII tree instead of Mermaid (only with id). Default: false",
				},
			},
		},
	}, r.handleGraph)
}

func (r *Registry) handleGraph(args json.RawMessage) *ToolResult {
	var params struct {
		ID    int  `json:"id"`
		ASCII bool `json:"ascii"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &params)
	}

	pending, err := r.q.List()
	if err != nil {
		return errorResult(fmt.Sprintf("list pending: %v", err))
	}
	active, err := r.q.Active()
	if err != nil {
		return errorResult(fmt.Sprintf("list active: %v", err))
	}
	done, err := r.q.ListDone()
	if err != nil {
		return errorResult(fmt.Sprintf("list done: %v", err))
	}

	nodes := queue.BuildGraph(pending, active, done)
	if len(nodes) == 0 {
		return textResult("No tasks in queue.")
	}

	if params.ID > 0 {
		sub := queue.Subgraph(nodes, params.ID)
		if sub == nil {
			return errorResult(fmt.Sprintf("task %03d not found", params.ID))
		}
		if params.ASCII {
			return textResult(queue.RenderASCII(sub, params.ID))
		}
		return textResult(queue.RenderMermaid(sub))
	}

	return textResult(queue.RenderMermaid(nodes))
}
```

- [ ] **Step 3: Register and run tests**

Add `r.registerGraph()` in `NewRegistry()`.

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -run TestGraph -v -count=1`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tool_graph.go internal/mcp/tools.go internal/mcp/mcp_test.go
git commit -m "feat(mcp): add cubit_graph tool with Mermaid and ASCII rendering"
```

---

### Task 12: cubit_status tool

**Files:**
- Create: `internal/mcp/tool_status.go`
- Modify: `internal/mcp/tools.go` (add `r.registerStatus()`)

- [ ] **Step 1: Write the failing test**

Add to `internal/mcp/mcp_test.go`:

```go
func TestStatus(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("task A", queue.CreateOptions{})
	srv.q.Create("task B", queue.CreateOptions{})
	srv.q.Create("task C", queue.CreateOptions{DependsOn: []int{1}})
	srv.q.PopReady()

	result := callTool(t, srv, "cubit_status", nil)
	if result.IsError {
		t.Fatalf("cubit_status error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Pending: 2") {
		t.Errorf("expected 2 pending: %s", text)
	}
	if !strings.Contains(text, "Active: 1") {
		t.Errorf("expected 1 active: %s", text)
	}
	if !strings.Contains(text, "Ready: 1") {
		t.Errorf("expected 1 ready: %s", text)
	}
}
```

- [ ] **Step 2: Implement cubit_status**

```go
package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/SeanoChang/cubit/internal/queue"
)

func (r *Registry) registerStatus() {
	r.register(ToolDef{
		Name:        "cubit_status",
		Description: "Show queue health: pending/active/done counts, ready task count, and brief token size.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, r.handleStatus)
}

func (r *Registry) handleStatus(args json.RawMessage) *ToolResult {
	pending, err := r.q.List()
	if err != nil {
		return errorResult(fmt.Sprintf("list pending: %v", err))
	}
	active, err := r.q.Active()
	if err != nil {
		return errorResult(fmt.Sprintf("list active: %v", err))
	}
	done, err := r.q.ListDone()
	if err != nil {
		return errorResult(fmt.Sprintf("list done: %v", err))
	}

	ready := queue.ReadyNodes(pending, active, done)

	var lines []string
	lines = append(lines, fmt.Sprintf("Pending: %d", len(pending)))
	lines = append(lines, fmt.Sprintf("Active: %d", len(active)))
	lines = append(lines, fmt.Sprintf("Done: %d", len(done)))
	lines = append(lines, fmt.Sprintf("Ready: %d", len(ready)))

	// Brief token estimate
	agentDir := r.cfg.AgentDir()
	sections := brief.Sections(agentDir)
	totalTokens := 0
	for _, s := range sections {
		totalTokens += brief.EstimateTokens(s.Content)
	}
	lines = append(lines, fmt.Sprintf("Brief: ~%d tokens", totalTokens))

	return textResult(strings.Join(lines, "\n"))
}
```

- [ ] **Step 3: Register and run tests**

Add `r.registerStatus()` in `NewRegistry()`.

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/mcp/ -run TestStatus -v -count=1`
Expected: PASS

- [ ] **Step 4: Run full test suite**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./... -count=1`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/tool_status.go internal/mcp/tools.go internal/mcp/mcp_test.go
git commit -m "feat(mcp): add cubit_status tool with queue health and token counts"
```

---

### Task 13: Final integration verification

- [ ] **Step 1: Build and verify binary**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go build -o cubit . && ./cubit mcp --help`
Expected: shows help for `cubit mcp` command

- [ ] **Step 2: Manual smoke test**

Run a quick echo-pipe test to verify the MCP server responds:

```bash
cd /Users/seanochang/dev/projects/agents/cubit
echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | ./cubit mcp 2>/dev/null | head -1 | python3 -m json.tool
```

Expected: JSON response with `serverInfo.name = "cubit"` and `tools` capability.

```bash
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | ./cubit mcp 2>/dev/null | tail -1 | python3 -m json.tool
```

Expected: JSON with array of 8 tools (cubit_queue, cubit_todo, cubit_do, cubit_done, cubit_requeue, cubit_log, cubit_graph, cubit_status).

- [ ] **Step 3: Verify all tests pass**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./... -v -count=1`
Expected: all PASS

- [ ] **Step 4: Final commit (if any cleanup needed)**

---

## Summary

After completing all tasks, `cubit mcp` is a working MCP server that:

1. Starts on stdio, speaks JSON-RPC 2.0
2. Exposes 8 tools: `cubit_queue`, `cubit_todo`, `cubit_do`, `cubit_done`, `cubit_requeue`, `cubit_log`, `cubit_graph`, `cubit_status`
3. All tools delegate to existing `internal/queue` and `internal/brief` — zero logic duplication
4. Follows the existing `Register()` pattern in `cmd/mcp/`
5. Ready for Claude Code integration via `mcpServers` config

**Not included (Phase 2+):** `cubit_drain`, `LeafNodes()`, scoped worker memory, post-drain lifecycle, Discord.
