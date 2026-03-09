package queue

import (
	"strings"
	"testing"
)

func TestBuildGraph_AllStatuses(t *testing.T) {
	done := []*Task{
		{ID: 1, Title: "arch sweep", Mode: "loop", Status: "done", DependsOn: []int{}},
	}
	active := []*Task{{ID: 2, Title: "ablation", Mode: "loop", Status: "doing", DependsOn: []int{1}}}
	pending := []*Task{
		{ID: 3, Title: "write report", Mode: "once", Status: "pending", DependsOn: []int{1}},
		{ID: 4, Title: "publish", Mode: "once", Status: "pending", DependsOn: []int{2, 3}},
		{ID: 5, Title: "deploy", Mode: "once", Status: "pending", DependsOn: []int{}},
	}

	nodes := BuildGraph(pending, active, done)

	want := map[int]NodeStatus{
		1: StatusDone,
		2: StatusActive,
		3: StatusReady,   // dep 1 is done
		4: StatusWaiting, // dep 2 is active (not done)
		5: StatusReady,   // no deps
	}
	if len(nodes) != 5 {
		t.Fatalf("want 5 nodes, got %d", len(nodes))
	}
	for _, n := range nodes {
		if n.Status != want[n.Task.ID] {
			t.Errorf("node %d: status = %q, want %q", n.Task.ID, n.Status, want[n.Task.ID])
		}
	}
}

func TestBuildGraph_NoActive(t *testing.T) {
	pending := []*Task{
		{ID: 1, Title: "standalone", Mode: "once", Status: "pending", DependsOn: []int{}},
	}

	nodes := BuildGraph(pending, nil, nil)

	if len(nodes) != 1 {
		t.Fatalf("want 1 node, got %d", len(nodes))
	}
	if nodes[0].Status != StatusReady {
		t.Errorf("status = %q, want %q", nodes[0].Status, StatusReady)
	}
	if nodes[0].Task.ID != 1 {
		t.Errorf("task ID = %d, want 1", nodes[0].Task.ID)
	}
}

func TestBuildGraph_OrderByID(t *testing.T) {
	done := []*Task{
		{ID: 10, Title: "last done", Mode: "once", Status: "done", DependsOn: []int{}},
	}
	active := []*Task{{ID: 5, Title: "mid active", Mode: "once", Status: "doing", DependsOn: []int{}}}
	pending := []*Task{
		{ID: 20, Title: "high pending", Mode: "once", Status: "pending", DependsOn: []int{}},
		{ID: 1, Title: "low pending", Mode: "once", Status: "pending", DependsOn: []int{}},
	}

	nodes := BuildGraph(pending, active, done)

	if len(nodes) != 4 {
		t.Fatalf("want 4 nodes, got %d", len(nodes))
	}

	wantOrder := []int{1, 5, 10, 20}
	for i, n := range nodes {
		if n.Task.ID != wantOrder[i] {
			t.Errorf("position %d: got ID %d, want %d", i, n.Task.ID, wantOrder[i])
		}
	}
}

func TestBuildGraph_EmptyGraph(t *testing.T) {
	nodes := BuildGraph(nil, nil, nil)

	if len(nodes) != 0 {
		t.Fatalf("want 0 nodes, got %d", len(nodes))
	}
}

func TestBuildGraph_WaitingOnActive(t *testing.T) {
	active := []*Task{{ID: 1, Title: "active task", Mode: "once", Status: "doing", DependsOn: []int{}}}
	pending := []*Task{
		{ID: 2, Title: "depends on active", Mode: "once", Status: "pending", DependsOn: []int{1}},
	}

	nodes := BuildGraph(pending, active, nil)

	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(nodes))
	}

	wantStatus := map[int]NodeStatus{
		1: StatusActive,
		2: StatusWaiting, // dep 1 is active, not done
	}
	for _, n := range nodes {
		if n.Status != wantStatus[n.Task.ID] {
			t.Errorf("node %d: status = %q, want %q", n.Task.ID, n.Status, wantStatus[n.Task.ID])
		}
	}
}

func TestBuildGraph_MultipleActive(t *testing.T) {
	done := []*Task{
		{ID: 1, Title: "setup", Mode: "once", Status: "done", DependsOn: []int{}},
	}
	active := []*Task{
		{ID: 2, Title: "task a", Mode: "once", Status: "doing", DependsOn: []int{1}},
		{ID: 3, Title: "task b", Mode: "once", Status: "doing", DependsOn: []int{1}},
	}
	pending := []*Task{
		{ID: 4, Title: "merge", Mode: "once", Status: "pending", DependsOn: []int{2, 3}},
	}

	nodes := BuildGraph(pending, active, done)

	want := map[int]NodeStatus{
		1: StatusDone,
		2: StatusActive,
		3: StatusActive,
		4: StatusWaiting,
	}
	if len(nodes) != 4 {
		t.Fatalf("want 4 nodes, got %d", len(nodes))
	}
	for _, n := range nodes {
		if n.Status != want[n.Task.ID] {
			t.Errorf("node %d: status = %q, want %q", n.Task.ID, n.Status, want[n.Task.ID])
		}
	}
}

func TestReadyNodes_BasicDAG(t *testing.T) {
	done := []*Task{
		{ID: 1, Title: "setup", Mode: "once", Status: "done", DependsOn: []int{}},
	}
	active := []*Task{
		{ID: 2, Title: "running", Mode: "once", Status: "doing", DependsOn: []int{1}},
	}
	pending := []*Task{
		{ID: 3, Title: "also ready", Mode: "once", Status: "pending", DependsOn: []int{1}},
		{ID: 4, Title: "waiting", Mode: "once", Status: "pending", DependsOn: []int{2, 3}},
	}

	ready := ReadyNodes(pending, active, done)

	if len(ready) != 1 {
		t.Fatalf("want 1 ready node, got %d", len(ready))
	}
	if ready[0].ID != 3 {
		t.Errorf("ready[0].ID = %d, want 3", ready[0].ID)
	}
}

func TestReadyNodes_MultipleFanOut(t *testing.T) {
	done := []*Task{
		{ID: 1, Title: "root", Mode: "once", Status: "done", DependsOn: []int{}},
	}
	pending := []*Task{
		{ID: 2, Title: "branch a", Mode: "once", Status: "pending", DependsOn: []int{1}},
		{ID: 3, Title: "branch b", Mode: "once", Status: "pending", DependsOn: []int{1}},
		{ID: 4, Title: "branch c", Mode: "once", Status: "pending", DependsOn: []int{1}},
	}

	ready := ReadyNodes(pending, nil, done)

	if len(ready) != 3 {
		t.Fatalf("want 3 ready nodes, got %d", len(ready))
	}
}

func TestReadyNodes_NoDeps(t *testing.T) {
	pending := []*Task{
		{ID: 1, Title: "a", Mode: "once", Status: "pending", DependsOn: []int{}},
		{ID: 2, Title: "b", Mode: "once", Status: "pending", DependsOn: []int{}},
	}

	ready := ReadyNodes(pending, nil, nil)
	if len(ready) != 2 {
		t.Fatalf("want 2 ready, got %d", len(ready))
	}
}

func TestReadyNodes_Empty(t *testing.T) {
	ready := ReadyNodes(nil, nil, nil)
	if len(ready) != 0 {
		t.Fatalf("want 0 ready, got %d", len(ready))
	}
}

func TestGraphComplete_AllDone(t *testing.T) {
	done := []*Task{
		{ID: 1, Status: "done"},
		{ID: 2, Status: "done"},
	}
	if !GraphComplete(nil, nil, done) {
		t.Error("expected complete when all done and nothing pending/active")
	}
}

func TestGraphComplete_StillPending(t *testing.T) {
	done := []*Task{{ID: 1, Status: "done"}}
	pending := []*Task{{ID: 2, Status: "pending", DependsOn: []int{1}}}
	if GraphComplete(pending, nil, done) {
		t.Error("expected not complete with pending tasks")
	}
}

func TestGraphComplete_StillActive(t *testing.T) {
	active := []*Task{{ID: 1, Status: "doing"}}
	if GraphComplete(nil, active, nil) {
		t.Error("expected not complete with active tasks")
	}
}

func TestGraphComplete_EmptyGraph(t *testing.T) {
	if !GraphComplete(nil, nil, nil) {
		t.Error("empty graph should be complete")
	}
}

func TestDetectCycle_NoCycle(t *testing.T) {
	nodes := []*GraphNode{
		{Task: &Task{ID: 1, DependsOn: []int{}}},
		{Task: &Task{ID: 2, DependsOn: []int{1}}},
		{Task: &Task{ID: 3, DependsOn: []int{2}}},
	}
	if err := DetectCycle(nodes); err != nil {
		t.Errorf("expected no cycle, got: %v", err)
	}
}

func TestDetectCycle_DirectCycle(t *testing.T) {
	nodes := []*GraphNode{
		{Task: &Task{ID: 1, DependsOn: []int{2}}},
		{Task: &Task{ID: 2, DependsOn: []int{1}}},
	}
	if err := DetectCycle(nodes); err == nil {
		t.Error("expected cycle error, got nil")
	}
}

func TestDetectCycle_IndirectCycle(t *testing.T) {
	nodes := []*GraphNode{
		{Task: &Task{ID: 1, DependsOn: []int{3}}},
		{Task: &Task{ID: 2, DependsOn: []int{1}}},
		{Task: &Task{ID: 3, DependsOn: []int{2}}},
	}
	if err := DetectCycle(nodes); err == nil {
		t.Error("expected cycle error, got nil")
	}
}

func TestDetectCycle_DiamondNoCycle(t *testing.T) {
	nodes := []*GraphNode{
		{Task: &Task{ID: 1, DependsOn: []int{}}},
		{Task: &Task{ID: 2, DependsOn: []int{1}}},
		{Task: &Task{ID: 3, DependsOn: []int{1}}},
		{Task: &Task{ID: 4, DependsOn: []int{2, 3}}},
	}
	if err := DetectCycle(nodes); err != nil {
		t.Errorf("expected no cycle in diamond, got: %v", err)
	}
}

func TestSubgraph_Center(t *testing.T) {
	// 001 → 003 → 004, 002 → 004 (unrelated branch)
	nodes := []*GraphNode{
		{Task: &Task{ID: 1, DependsOn: []int{}}, Status: StatusDone},
		{Task: &Task{ID: 2, DependsOn: []int{}}, Status: StatusDone},
		{Task: &Task{ID: 3, DependsOn: []int{1}}, Status: StatusReady},
		{Task: &Task{ID: 4, DependsOn: []int{2, 3}}, Status: StatusWaiting},
	}
	sub := Subgraph(nodes, 3)
	ids := make(map[int]bool)
	for _, n := range sub {
		ids[n.Task.ID] = true
	}
	// Should include: 1 (ancestor), 3 (center), 4 (descendant). NOT 2.
	if !ids[1] || !ids[3] || !ids[4] {
		t.Errorf("subgraph missing expected nodes, got IDs: %v", ids)
	}
	if ids[2] {
		t.Error("subgraph should not include task 2 (unrelated branch)")
	}
}

func TestSubgraph_NotFound(t *testing.T) {
	nodes := []*GraphNode{
		{Task: &Task{ID: 1, DependsOn: []int{}}},
	}
	sub := Subgraph(nodes, 99)
	if sub != nil {
		t.Errorf("expected nil for missing id, got %v", sub)
	}
}

func TestRenderMermaid_ContainsNodes(t *testing.T) {
	nodes := []*GraphNode{
		{Task: &Task{ID: 1, Title: "first", Mode: "once", DependsOn: []int{}}, Status: StatusDone},
		{Task: &Task{ID: 2, Title: "second", Mode: "loop", DependsOn: []int{1}}, Status: StatusReady},
	}
	out := RenderMermaid(nodes)
	if !strings.Contains(out, "graph TD") {
		t.Error("missing graph TD header")
	}
	if !strings.Contains(out, "001") || !strings.Contains(out, "002") {
		t.Error("missing node IDs")
	}
	if !strings.Contains(out, "001 --> 002") {
		t.Error("missing edge 001 --> 002")
	}
}

func TestRenderASCII_ContainsSections(t *testing.T) {
	nodes := []*GraphNode{
		{Task: &Task{ID: 1, Title: "root", Mode: "once", DependsOn: []int{}}, Status: StatusDone},
		{Task: &Task{ID: 2, Title: "middle", Mode: "once", DependsOn: []int{1}}, Status: StatusReady},
		{Task: &Task{ID: 3, Title: "leaf", Mode: "once", DependsOn: []int{2}}, Status: StatusWaiting},
	}
	out := RenderASCII(nodes, 2)
	if !strings.Contains(out, "Depends on") {
		t.Error("missing 'Depends on' section")
	}
	if !strings.Contains(out, "Blocks") {
		t.Error("missing 'Blocks' section")
	}
	if !strings.Contains(out, "middle") {
		t.Error("missing center task title")
	}
}
