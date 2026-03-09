package queue

import "strings"

// GoalMet returns true if the output contains the GOAL_MET signal.
func GoalMet(output string) bool {
	return strings.Contains(output, "GOAL_MET")
}
