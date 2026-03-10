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

	agentDir := r.cfg.AgentDir()
	sections := brief.Sections(agentDir)
	totalTokens := 0
	for _, s := range sections {
		totalTokens += brief.EstimateTokens(s.Content)
	}
	lines = append(lines, fmt.Sprintf("Brief: ~%d tokens", totalTokens))

	return textResult(strings.Join(lines, "\n"))
}
