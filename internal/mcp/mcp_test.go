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
		t.Fatalf("tools/call RPC error: %s", resp.Error.Message)
	}
	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	json.Unmarshal(data, &result)
	return result
}

// --- Protocol tests ---

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
	s := string(data)
	for _, tool := range []string{"cubit_queue", "cubit_todo", "cubit_do", "cubit_done", "cubit_requeue", "cubit_log", "cubit_graph", "cubit_status"} {
		if !strings.Contains(s, tool) {
			t.Errorf("tools/list missing %s", tool)
		}
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

func TestUnknownTool(t *testing.T) {
	srv, _ := setupTest(t)
	result := callTool(t, srv, "cubit_bogus", nil)
	if !result.IsError {
		t.Error("expected IsError for unknown tool")
	}
}

// --- cubit_queue tests ---

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

func TestQueueFilterDone(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("task A", queue.CreateOptions{})
	srv.q.PopReady()
	srv.q.CompleteByID(1, "finished")

	result := callTool(t, srv, "cubit_queue", map[string]string{"filter": "done"})
	text := result.Content[0].Text
	if !strings.Contains(text, "001") || !strings.Contains(text, "task A") {
		t.Errorf("missing done task: %s", text)
	}
}

// --- cubit_todo tests ---

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

// --- cubit_do tests ---

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

func TestDoPopAll(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("A", queue.CreateOptions{})
	srv.q.Create("B", queue.CreateOptions{})
	srv.q.Create("blocked", queue.CreateOptions{DependsOn: []int{1, 2}})

	result := callTool(t, srv, "cubit_do", map[string]any{"all": true})
	if result.IsError {
		t.Fatalf("cubit_do all error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Popped 2") {
		t.Errorf("expected 2 popped: %s", text)
	}
}

// --- cubit_done tests ---

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

// --- cubit_requeue tests ---

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

// --- cubit_log tests ---

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

// --- cubit_graph tests ---

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

func TestGraphEmpty(t *testing.T) {
	srv, _ := setupTest(t)
	result := callTool(t, srv, "cubit_graph", nil)
	if result.IsError {
		t.Fatal("expected no error on empty graph")
	}
	if !strings.Contains(result.Content[0].Text, "No tasks") {
		t.Errorf("expected empty message: %s", result.Content[0].Text)
	}
}

// --- cubit_drain tests ---

func TestDrainRejectsMultipleLeaves(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("branch A", queue.CreateOptions{})
	srv.q.Create("branch B", queue.CreateOptions{})

	result := callTool(t, srv, "cubit_drain", nil)
	if !result.IsError {
		t.Fatal("expected error for multiple terminal nodes")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "2 terminal nodes") {
		t.Errorf("error should mention count: %s", text)
	}
	if !strings.Contains(text, "branch A") || !strings.Contains(text, "branch B") {
		t.Errorf("error should list leaf names: %s", text)
	}
}

func TestDrainRejectsEmpty(t *testing.T) {
	srv, _ := setupTest(t)
	result := callTool(t, srv, "cubit_drain", nil)
	if !result.IsError {
		t.Fatal("expected error for empty DAG")
	}
	if !strings.Contains(result.Content[0].Text, "no tasks") {
		t.Errorf("expected 'no tasks' error: %s", result.Content[0].Text)
	}
}

func TestDrainAcceptsSingleTerminal(t *testing.T) {
	srv, _ := setupTest(t)
	srv.q.Create("root", queue.CreateOptions{})
	srv.q.Create("terminal", queue.CreateOptions{DependsOn: []int{1}})

	result := callTool(t, srv, "cubit_drain", nil)
	// Will try to spawn cubit run --once — may fail if binary not in PATH
	// but validation should pass (no IsError from validation)
	text := result.Content[0].Text
	if result.IsError && strings.Contains(text, "terminal nodes") {
		t.Errorf("validation should pass for single terminal: %s", text)
	}
}

// --- cubit_status tests ---

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
