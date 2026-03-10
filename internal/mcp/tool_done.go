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
