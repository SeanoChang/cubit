package mcp

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"syscall"

	"github.com/SeanoChang/cubit/internal/queue"
)

func (r *Registry) registerDrain() {
	r.register(ToolDef{
		Name:        "cubit_drain",
		Description: "Signal that planning is complete and start DAG execution. Validates the DAG has exactly one terminal node, then spawns a detached cubit run --once process. Returns immediately — execution runs in the background. If validation fails, add a summary node that depends on all leaf tasks and try again.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, r.handleDrain)
}

func (r *Registry) handleDrain(args json.RawMessage) *ToolResult {
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

	if err := queue.ValidateTerminal(pending, active, done); err != nil {
		return errorResult(err.Error())
	}

	cmd := exec.Command("cubit", "run", "--once")
	cmd.Env = cleanEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return errorResult(fmt.Sprintf("failed to start drain: %v", err))
	}

	ready := queue.ReadyNodes(pending, active, done)
	return textResult(fmt.Sprintf("Drain started (pid %d), %d tasks ready", cmd.Process.Pid, len(ready)))
}
