package delegation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DelegationStatus string

const (
	StatusPending DelegationStatus = "pending"
	StatusPartial DelegationStatus = "partial"
	StatusReady   DelegationStatus = "ready"
	StatusDone    DelegationStatus = "done"
)

type SubTaskStatus string

const (
	SubStatusPending  SubTaskStatus = "pending"
	SubStatusComplete SubTaskStatus = "complete"
	SubStatusRejected SubTaskStatus = "rejected"
)

type SubTask struct {
	To             string        `json:"to"`
	Task           string        `json:"task"`
	Status         SubTaskStatus `json:"status"`
	DispatchedMail string        `json:"dispatched_mail"`
	ResponseMail   string        `json:"response_mail,omitempty"`
	Attempts       int           `json:"attempts"`
}

type Delegation struct {
	ID          string           `json:"id"`
	Created     string           `json:"created"`
	Owner       string           `json:"owner"`
	GoalContext string           `json:"goal_context"`
	OnComplete  string           `json:"on_complete"`
	Status      DelegationStatus `json:"status"`
	SubTasks    []SubTask        `json:"sub_tasks"`
}

// Recalculate updates the delegation status based on sub-task statuses.
func (d *Delegation) Recalculate() {
	complete := 0
	for _, st := range d.SubTasks {
		if st.Status == SubStatusComplete {
			complete++
		}
	}
	switch {
	case complete == len(d.SubTasks):
		d.Status = StatusReady
	case complete > 0:
		d.Status = StatusPartial
	default:
		d.Status = StatusPending
	}
}

// Read loads a delegation from a JSON file.
func Read(path string) (*Delegation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read delegation: %w", err)
	}
	var d Delegation
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse delegation: %w", err)
	}
	return &d, nil
}

// Write atomically writes the delegation to its directory.
// Layout: <baseDir>/<id>/delegation.json with a responses/ subdirectory.
func Write(baseDir string, d *Delegation) error {
	delDir := filepath.Join(baseDir, d.ID)
	if err := os.MkdirAll(delDir, 0o755); err != nil {
		return fmt.Errorf("create delegation dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(delDir, "responses"), 0o755); err != nil {
		return fmt.Errorf("create responses dir: %w", err)
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal delegation: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(delDir, "delegation.json")
	tmp := filepath.Join(delDir, ".tmp-delegation.json")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// FindByID loads a delegation from <baseDir>/<id>/delegation.json.
func FindByID(baseDir, id string) (*Delegation, string, error) {
	path := filepath.Join(baseDir, id, "delegation.json")
	d, err := Read(path)
	if err != nil {
		return nil, "", err
	}
	return d, path, nil
}
