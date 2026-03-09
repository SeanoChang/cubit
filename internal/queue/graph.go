package queue

import (
	"fmt"
	"sort"
	"strings"
)

// NodeStatus describes where a task sits in the graph.
type NodeStatus string

const (
	StatusDone    NodeStatus = "done"
	StatusActive  NodeStatus = "active"
	StatusReady   NodeStatus = "ready"
	StatusWaiting NodeStatus = "waiting"
)

// GraphNode pairs a task with its computed status.
type GraphNode struct {
	Task   *Task
	Status NodeStatus
}

// BuildGraph assembles all tasks into an ordered slice of GraphNodes.
// pending: tasks in queue/*.md
// active: task in queue/.doing (nil if none)
// done: tasks in queue/done/*.md (nil if none)
func BuildGraph(pending []*Task, active *Task, done []*Task) []*GraphNode {
	doneIDs := make(map[int]bool)
	for _, t := range done {
		doneIDs[t.ID] = true
	}

	var nodes []*GraphNode

	for _, t := range done {
		nodes = append(nodes, &GraphNode{Task: t, Status: StatusDone})
	}

	if active != nil {
		nodes = append(nodes, &GraphNode{Task: active, Status: StatusActive})
	}

	for _, t := range pending {
		status := StatusReady
		for _, dep := range t.DependsOn {
			if !doneIDs[dep] {
				status = StatusWaiting
				break
			}
		}
		nodes = append(nodes, &GraphNode{Task: t, Status: status})
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Task.ID < nodes[j].Task.ID
	})

	return nodes
}

// Subgraph returns the subset of nodes that are ancestors or descendants of
// the given task ID, plus the task itself. Returns nil if id not found.
func Subgraph(nodes []*GraphNode, id int) []*GraphNode {
	// index by ID
	byID := make(map[int]*GraphNode, len(nodes))
	for _, n := range nodes {
		byID[n.Task.ID] = n
	}
	if _, ok := byID[id]; !ok {
		return nil
	}

	// forward: id depends on these (ancestors)
	// reverse: these depend on id (descendants)
	reverse := make(map[int][]int) // id → list of task IDs that depend on it
	for _, n := range nodes {
		for _, dep := range n.Task.DependsOn {
			reverse[dep] = append(reverse[dep], n.Task.ID)
		}
	}

	visited := make(map[int]bool)
	var walk func(cur int, forward bool)
	walk = func(cur int, forward bool) {
		if visited[cur] {
			return
		}
		visited[cur] = true
		if forward {
			// walk ancestors (deps of cur)
			if n, ok := byID[cur]; ok {
				for _, dep := range n.Task.DependsOn {
					walk(dep, true)
				}
			}
		} else {
			// walk descendants (tasks that depend on cur)
			for _, child := range reverse[cur] {
				walk(child, false)
			}
		}
	}

	walk(id, true) // ancestors (marks id as visited)
	// Seed descendants directly: id is already visited so we bypass the guard
	// by starting from its children in the reverse map.
	for _, child := range reverse[id] {
		walk(child, false)
	}

	var result []*GraphNode
	for _, n := range nodes {
		if visited[n.Task.ID] {
			result = append(result, n)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Task.ID < result[j].Task.ID
	})
	return result
}

// RenderMermaid renders nodes as a Mermaid graph TD diagram.
// Edges only drawn between nodes present in the slice.
func RenderMermaid(nodes []*GraphNode) string {
	inSet := make(map[int]bool, len(nodes))
	for _, n := range nodes {
		inSet[n.Task.ID] = true
	}

	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Node definitions
	for _, n := range nodes {
		t := n.Task
		fmt.Fprintf(&sb, "  %03d[\"%s [%s]\"]:::%s\n", t.ID, t.Title, t.Mode, string(n.Status))
	}

	// Edges: dep --> node (dep must be done before node)
	for _, n := range nodes {
		for _, dep := range n.Task.DependsOn {
			if inSet[dep] {
				fmt.Fprintf(&sb, "  %03d --> %03d\n", dep, n.Task.ID)
			}
		}
	}

	// Class definitions
	sb.WriteString("  classDef done fill:#2d6a4f,color:#fff\n")
	sb.WriteString("  classDef active fill:#1d4e89,color:#fff\n")
	sb.WriteString("  classDef ready fill:#495057,color:#fff\n")
	sb.WriteString("  classDef waiting fill:#856404,color:#fff\n")

	return sb.String()
}

// RenderASCII renders a task-centered ASCII tree showing ancestors and descendants.
// centerID is the focal task. nodes should already be the subgraph slice.
func RenderASCII(nodes []*GraphNode, centerID int) string {
	byID := make(map[int]*GraphNode, len(nodes))
	for _, n := range nodes {
		byID[n.Task.ID] = n
	}

	center, ok := byID[centerID]
	if !ok {
		return ""
	}

	// Build reverse map within the subgraph
	reverse := make(map[int][]int)
	for _, n := range nodes {
		for _, dep := range n.Task.DependsOn {
			if _, exists := byID[dep]; exists {
				reverse[dep] = append(reverse[dep], n.Task.ID)
			}
		}
	}

	nodeLabel := func(n *GraphNode) string {
		t := n.Task
		return fmt.Sprintf("%03d [%s] %s  %s", t.ID, t.Mode, t.Title, statusSymbol(n.Status))
	}

	treeLines := func(ids []int, prefix string) []string {
		var lines []string
		for i, id := range ids {
			n := byID[id]
			if n == nil {
				continue
			}
			connector := "├── "
			if i == len(ids)-1 {
				connector = "└── "
			}
			lines = append(lines, prefix+connector+nodeLabel(n))
		}
		return lines
	}

	var sb strings.Builder

	// Ancestors (deps of center)
	if len(center.Task.DependsOn) > 0 {
		sb.WriteString("Depends on:\n")
		for _, line := range treeLines(center.Task.DependsOn, "  ") {
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// Center task
	sb.WriteString(nodeLabel(center) + "\n")

	// Descendants (tasks that depend on center)
	if deps := reverse[centerID]; len(deps) > 0 {
		sort.Ints(deps)
		sb.WriteString("\nBlocks:\n")
		for _, line := range treeLines(deps, "  ") {
			sb.WriteString(line + "\n")
		}
	}

	return sb.String()
}

func statusSymbol(s NodeStatus) string {
	switch s {
	case StatusDone:
		return "→ DONE"
	case StatusActive:
		return "← ACTIVE"
	case StatusReady:
		return "✓ ready"
	case StatusWaiting:
		return "⏳ waiting"
	default:
		return ""
	}
}

// DetectCycle checks for circular dependencies using DFS coloring.
// Returns an error if a cycle is found.
func DetectCycle(nodes []*GraphNode) error {
	adj := make(map[int][]int)
	for _, n := range nodes {
		adj[n.Task.ID] = n.Task.DependsOn
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[int]int)

	var visit func(id int) error
	visit = func(id int) error {
		color[id] = gray
		for _, dep := range adj[id] {
			if color[dep] == gray {
				return fmt.Errorf("circular dependency detected: %d → %d", id, dep)
			}
			if color[dep] == white {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}

	for _, n := range nodes {
		if color[n.Task.ID] == white {
			if err := visit(n.Task.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
