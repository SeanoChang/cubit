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
