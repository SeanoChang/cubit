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
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult(fmt.Sprintf("invalid arguments: %v", err))
		}
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
