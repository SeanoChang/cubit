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
	q        *queue.Queue
	cfg      *config.Config
	defs     []ToolDef
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
	r.registerTodo()
	r.registerDo()
	r.registerDone()
	r.registerRequeue()
	r.registerLog()
	r.registerGraph()
	r.registerStatus()
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
