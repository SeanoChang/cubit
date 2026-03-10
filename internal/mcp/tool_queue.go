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
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult(fmt.Sprintf("invalid arguments: %v", err))
		}
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
