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
