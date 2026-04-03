package delegation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDelegationJSON(t *testing.T) {
	orig := Delegation{
		ID:          "del-001",
		Created:     "2026-04-02T10:00:00Z",
		Owner:       "noah",
		GoalContext: "deploy the new feature",
		OnComplete:  "notify owner",
		Status:      StatusPending,
		SubTasks: []SubTask{
			{
				To:             "aria",
				Task:           "write tests",
				Status:         SubStatusPending,
				DispatchedMail: "mail-001.md",
				Attempts:       1,
			},
			{
				To:             "felix",
				Task:           "review code",
				Status:         SubStatusComplete,
				DispatchedMail: "mail-002.md",
				ResponseMail:   "resp-002.md",
				Attempts:       2,
			},
		},
	}

	data, err := json.Marshal(&orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Delegation
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != orig.ID {
		t.Errorf("ID: got %q, want %q", got.ID, orig.ID)
	}
	if got.Created != orig.Created {
		t.Errorf("Created: got %q, want %q", got.Created, orig.Created)
	}
	if got.Owner != orig.Owner {
		t.Errorf("Owner: got %q, want %q", got.Owner, orig.Owner)
	}
	if got.GoalContext != orig.GoalContext {
		t.Errorf("GoalContext: got %q, want %q", got.GoalContext, orig.GoalContext)
	}
	if got.OnComplete != orig.OnComplete {
		t.Errorf("OnComplete: got %q, want %q", got.OnComplete, orig.OnComplete)
	}
	if got.Status != orig.Status {
		t.Errorf("Status: got %q, want %q", got.Status, orig.Status)
	}
	if len(got.SubTasks) != len(orig.SubTasks) {
		t.Fatalf("SubTasks length: got %d, want %d", len(got.SubTasks), len(orig.SubTasks))
	}
	for i, st := range got.SubTasks {
		if st.To != orig.SubTasks[i].To {
			t.Errorf("SubTask[%d].To: got %q, want %q", i, st.To, orig.SubTasks[i].To)
		}
		if st.Task != orig.SubTasks[i].Task {
			t.Errorf("SubTask[%d].Task: got %q, want %q", i, st.Task, orig.SubTasks[i].Task)
		}
		if st.Status != orig.SubTasks[i].Status {
			t.Errorf("SubTask[%d].Status: got %q, want %q", i, st.Status, orig.SubTasks[i].Status)
		}
		if st.DispatchedMail != orig.SubTasks[i].DispatchedMail {
			t.Errorf("SubTask[%d].DispatchedMail: got %q, want %q", i, st.DispatchedMail, orig.SubTasks[i].DispatchedMail)
		}
		if st.ResponseMail != orig.SubTasks[i].ResponseMail {
			t.Errorf("SubTask[%d].ResponseMail: got %q, want %q", i, st.ResponseMail, orig.SubTasks[i].ResponseMail)
		}
		if st.Attempts != orig.SubTasks[i].Attempts {
			t.Errorf("SubTask[%d].Attempts: got %d, want %d", i, st.Attempts, orig.SubTasks[i].Attempts)
		}
	}
}

func TestRecalculate(t *testing.T) {
	tests := []struct {
		name     string
		statuses []SubTaskStatus
		want     DelegationStatus
	}{
		{
			name:     "all pending",
			statuses: []SubTaskStatus{SubStatusPending, SubStatusPending, SubStatusPending},
			want:     StatusPending,
		},
		{
			name:     "one complete",
			statuses: []SubTaskStatus{SubStatusComplete, SubStatusPending, SubStatusPending},
			want:     StatusPartial,
		},
		{
			name:     "all complete",
			statuses: []SubTaskStatus{SubStatusComplete, SubStatusComplete, SubStatusComplete},
			want:     StatusReady,
		},
		{
			name:     "rejected resets to partial",
			statuses: []SubTaskStatus{SubStatusComplete, SubStatusRejected, SubStatusPending},
			want:     StatusPartial,
		},
		{
			name:     "single complete",
			statuses: []SubTaskStatus{SubStatusComplete},
			want:     StatusReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Delegation{ID: "test", Status: StatusPending}
			for _, s := range tt.statuses {
				d.SubTasks = append(d.SubTasks, SubTask{Status: s})
			}
			d.Recalculate()
			if d.Status != tt.want {
				t.Errorf("Recalculate() = %q, want %q", d.Status, tt.want)
			}
		})
	}
}

func TestWriteRead(t *testing.T) {
	tmpDir := t.TempDir()

	orig := &Delegation{
		ID:          "del-test-42",
		Created:     "2026-04-02T12:00:00Z",
		Owner:       "noah",
		GoalContext: "test write and read",
		OnComplete:  "signal done",
		Status:      StatusPending,
		SubTasks: []SubTask{
			{
				To:             "aria",
				Task:           "do something",
				Status:         SubStatusPending,
				DispatchedMail: "mail-099.md",
				Attempts:       0,
			},
		},
	}

	// Write the delegation.
	if err := Write(tmpDir, orig); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify delegation.json exists.
	jsonPath := filepath.Join(tmpDir, orig.ID, "delegation.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("delegation.json not found: %v", err)
	}

	// Verify responses/ directory exists.
	respDir := filepath.Join(tmpDir, orig.ID, "responses")
	info, err := os.Stat(respDir)
	if err != nil {
		t.Fatalf("responses dir not found: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("responses is not a directory")
	}

	// FindByID should read it back correctly.
	got, path, err := FindByID(tmpDir, orig.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if path != jsonPath {
		t.Errorf("path: got %q, want %q", path, jsonPath)
	}
	if got.ID != orig.ID {
		t.Errorf("ID: got %q, want %q", got.ID, orig.ID)
	}
	if got.Owner != orig.Owner {
		t.Errorf("Owner: got %q, want %q", got.Owner, orig.Owner)
	}
	if got.GoalContext != orig.GoalContext {
		t.Errorf("GoalContext: got %q, want %q", got.GoalContext, orig.GoalContext)
	}
	if got.Status != orig.Status {
		t.Errorf("Status: got %q, want %q", got.Status, orig.Status)
	}
	if len(got.SubTasks) != 1 {
		t.Fatalf("SubTasks length: got %d, want 1", len(got.SubTasks))
	}
	if got.SubTasks[0].To != "aria" {
		t.Errorf("SubTask[0].To: got %q, want %q", got.SubTasks[0].To, "aria")
	}
}
