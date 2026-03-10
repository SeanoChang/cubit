package task

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
	Use:   "graph [task-id]",
	Short: "Print the task DAG with dependency status",
	Long: `Without arguments: print a flat list of all tasks, optionally filtered.
With a task ID: print a subgraph view for that task (Mermaid by default, --ascii for tree).`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := getQ()

		statusFilter, _ := cmd.Flags().GetString("status")
		modeFilter, _ := cmd.Flags().GetString("mode")
		ascii, _ := cmd.Flags().GetBool("ascii")

		pending, err := q.List()
		if err != nil {
			return err
		}
		active, err := q.Active()
		if err != nil {
			return err
		}
		done, err := q.ListDone()
		if err != nil {
			return err
		}

		nodes := queue.BuildGraph(pending, active, done)

		if len(nodes) == 0 {
			fmt.Println("no tasks")
			return nil
		}

		cycleErr := queue.DetectCycle(nodes)

		// Subgraph view for a specific task ID
		if len(args) == 1 {
			if cycleErr != nil {
				return fmt.Errorf("cannot render subgraph: %w", cycleErr)
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid task id %q: must be a number", args[0])
			}
			sub := queue.Subgraph(nodes, id)
			if sub == nil {
				return fmt.Errorf("task %03d not found", id)
			}
			if ascii {
				fmt.Print(queue.RenderASCII(sub, id))
			} else {
				fmt.Print(queue.RenderMermaid(sub))
			}
			return nil
		}

		// Flat list with optional filters
		if cycleErr != nil {
			fmt.Printf("warning: %v\n\n", cycleErr)
		}
		filtered := applyFilters(nodes, statusFilter, modeFilter)
		if len(filtered) == 0 {
			fmt.Println("no tasks match filter")
			return nil
		}
		for _, n := range filtered {
			fmt.Println(formatNode(n))
		}
		return nil
	},
}

// applyFilters narrows nodes by status and/or mode CSV values.
func applyFilters(nodes []*queue.GraphNode, statusCSV, modeCSV string) []*queue.GraphNode {
	statuses := parseCSV(statusCSV)
	modes := parseCSV(modeCSV)

	if len(statuses) == 0 && len(modes) == 0 {
		return nodes
	}

	var out []*queue.GraphNode
	for _, n := range nodes {
		if len(statuses) > 0 && !statuses[string(n.Status)] {
			continue
		}
		if len(modes) > 0 && !modes[n.Task.Mode] {
			continue
		}
		out = append(out, n)
	}
	return out
}

func parseCSV(s string) map[string]bool {
	if s == "" {
		return nil
	}
	m := make(map[string]bool)
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			m[v] = true
		}
	}
	return m
}

func formatNode(n *queue.GraphNode) string {
	t := n.Task
	modeTag := fmt.Sprintf("[%s]", t.Mode)

	title := t.Title
	if len(title) > 30 {
		title = title[:27] + "..."
	}

	left := fmt.Sprintf("  %03d %-6s %-30s", t.ID, modeTag, title)

	var right string
	switch n.Status {
	case queue.StatusDone:
		right = "→ DONE"
	case queue.StatusActive:
		right = "← ACTIVE"
	case queue.StatusReady:
		right = "✓ ready"
	case queue.StatusWaiting:
		deps := make([]string, len(t.DependsOn))
		for i, d := range t.DependsOn {
			deps[i] = fmt.Sprintf("%03d", d)
		}
		right = fmt.Sprintf("⏳ waiting on [%s]", strings.Join(deps, ", "))
	}

	return fmt.Sprintf("%-48s %s", left, right)
}
