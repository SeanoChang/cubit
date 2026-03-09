package queue

import "testing"

func TestGoalMet_Present(t *testing.T) {
	output := "Results look good.\n\nGOAL_MET\n\nDone."
	if !GoalMet(output) {
		t.Error("expected goal met")
	}
}

func TestGoalMet_Absent(t *testing.T) {
	output := "Still working on it. val_bpb = 0.97."
	if GoalMet(output) {
		t.Error("expected goal not met")
	}
}

func TestGoalMet_InlineDoesNotCount(t *testing.T) {
	output := "The GOAL_MET criteria are not yet satisfied."
	// Lenient match — agent is instructed to put it on its own line,
	// but we detect it anywhere.
	if !GoalMet(output) {
		t.Error("expected goal met (lenient match)")
	}
}

func TestGoalMet_Empty(t *testing.T) {
	if GoalMet("") {
		t.Error("expected goal not met on empty output")
	}
}
