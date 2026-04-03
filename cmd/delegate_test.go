package cmd

import "testing"

func TestDelegationIDGeneration(t *testing.T) {
	id := generateDelegationID("research-task")
	if id == "" {
		t.Error("ID should not be empty")
	}
	if len(id) < 20 {
		t.Errorf("ID too short: %q", id)
	}
	if id[:4] != "del-" {
		t.Errorf("ID should start with del-, got %q", id)
	}
}
