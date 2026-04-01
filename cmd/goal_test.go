package cmd

import (
	"testing"
)

func TestParseGoals(t *testing.T) {
	input := `# Goals

## [2026-04-01 14:30] from seanoc
Research market trends

## [2026-04-01 14:35] from seanoc [background]
Check disk usage

## [2026-04-01 14:40] scheduled: daily-check [consolidation]
Reorganize memory files

## [2026-04-01 14:45] self-directed: deep dive
Multi-line body
with more detail here
`

	goals := parseGoals(input)

	if len(goals) != 4 {
		t.Fatalf("got %d goals, want 4", len(goals))
	}

	// Goal 1: session (default, no tag)
	if goals[0].goalType != "session" {
		t.Errorf("goal 1 type = %q, want session", goals[0].goalType)
	}
	if goals[0].source != "from seanoc" {
		t.Errorf("goal 1 source = %q, want 'from seanoc'", goals[0].source)
	}
	if goals[0].body != "Research market trends" {
		t.Errorf("goal 1 body = %q", goals[0].body)
	}

	// Goal 2: background
	if goals[1].goalType != "background" {
		t.Errorf("goal 2 type = %q, want background", goals[1].goalType)
	}
	if goals[1].source != "from seanoc" {
		t.Errorf("goal 2 source = %q, want 'from seanoc'", goals[1].source)
	}

	// Goal 3: consolidation with scheduled source
	if goals[2].goalType != "consolidation" {
		t.Errorf("goal 3 type = %q, want consolidation", goals[2].goalType)
	}
	if goals[2].source != "scheduled: daily-check" {
		t.Errorf("goal 3 source = %q, want 'scheduled: daily-check'", goals[2].source)
	}

	// Goal 4: session (no type tag), multi-line body
	if goals[3].goalType != "session" {
		t.Errorf("goal 4 type = %q, want session", goals[3].goalType)
	}
	if goals[3].body != "Multi-line body\nwith more detail here" {
		t.Errorf("goal 4 body = %q", goals[3].body)
	}
}

func TestParseGoalsEmpty(t *testing.T) {
	goals := parseGoals("# Goals\n\n<!-- Add goals here. -->\n")
	if len(goals) != 0 {
		t.Errorf("got %d goals from empty file, want 0", len(goals))
	}
}
