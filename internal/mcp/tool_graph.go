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
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult(fmt.Sprintf("invalid arguments: %v", err))
		}
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
