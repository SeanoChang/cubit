package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/SeanoChang/cubit/internal/queue"
)

func (r *Registry) registerTodo() {
	r.register(ToolDef{
		Name:        "cubit_todo",
		Description: "Create a new task in the queue. When creating a multi-step plan, always include a terminal summary node that depends on all leaf tasks. Use fan-out/fan-in patterns for parallel work.",
		InputSchema: map[string]any{
			"type":     "object",
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
