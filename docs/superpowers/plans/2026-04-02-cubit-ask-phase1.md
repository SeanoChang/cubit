# Cubit Ask Phase 1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `cubit ask write` and `cubit ask list` commands for structured inter-agent requests, backed by an `internal/ask/` package.

**Architecture:** New `internal/ask/` package holds types (Ask, AskContext, AskResponse) and file operations (Validate, GenerateID, Write, List, ListAll, Count). Thin `cmd/ask.go` wires cobra subcommands to the package. Follows existing patterns from `cmd/send.go` (inter-agent delivery) and `cmd/goal.go` (parent + child subcommands).

**Tech Stack:** Go 1.26, Cobra, standard library (`encoding/json`, `os`, `path/filepath`, `time`, `sort`)

---

### Task 1: Create `internal/ask/` package — types and validation

**Files:**
- Create: `internal/ask/ask.go`
- Create: `internal/ask/ask_test.go`

- [ ] **Step 1: Write failing tests for Validate**

Create `internal/ask/ask_test.go`:

```go
package ask

import "testing"

func TestValidate_AllFieldsPresent(t *testing.T) {
	a := &Ask{
		From:    "librarian",
		To:      "alice",
		Action:  "approve-merge",
		Options: []string{"approve", "deny"},
	}
	if err := Validate(a); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidate_MissingTo(t *testing.T) {
	a := &Ask{From: "librarian", Action: "approve-merge", Options: []string{"approve"}}
	if err := Validate(a); err == nil {
		t.Fatal("expected error for missing 'to'")
	}
}

func TestValidate_MissingFrom(t *testing.T) {
	a := &Ask{To: "alice", Action: "approve-merge", Options: []string{"approve"}}
	if err := Validate(a); err == nil {
		t.Fatal("expected error for missing 'from'")
	}
}

func TestValidate_MissingAction(t *testing.T) {
	a := &Ask{From: "librarian", To: "alice", Options: []string{"approve"}}
	if err := Validate(a); err == nil {
		t.Fatal("expected error for missing 'action'")
	}
}

func TestValidate_EmptyOptions(t *testing.T) {
	a := &Ask{From: "librarian", To: "alice", Action: "approve-merge", Options: []string{}}
	if err := Validate(a); err == nil {
		t.Fatal("expected error for empty options")
	}
}

func TestValidate_NilOptions(t *testing.T) {
	a := &Ask{From: "librarian", To: "alice", Action: "approve-merge"}
	if err := Validate(a); err == nil {
		t.Fatal("expected error for nil options")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/ask/ -v -run TestValidate`

Expected: compilation error — package/types don't exist yet.

- [ ] **Step 3: Write types and Validate**

Create `internal/ask/ask.go`:

```go
package ask

import "fmt"

// Ask represents a structured inter-agent request.
type Ask struct {
	ID        string       `json:"id"`
	From      string       `json:"from"`
	To        string       `json:"to"`
	Timestamp string       `json:"timestamp"`
	Action    string       `json:"action"`
	Context   AskContext   `json:"context"`
	Options   []string     `json:"options"`
	Response  *AskResponse `json:"response"`
}

// AskContext provides details about the request.
type AskContext struct {
	Reason         string   `json:"reason"`
	NoteIDs        []string `json:"note_ids,omitempty"`
	NoteTitles     []string `json:"note_titles,omitempty"`
	ProposedResult string   `json:"proposed_result,omitempty"`
}

// AskResponse is filled by the ask processor or human.
type AskResponse struct {
	Decision  string `json:"decision"`
	Reason    string `json:"reason"`
	Details   any    `json:"details"`
	Timestamp string `json:"timestamp"`
	Responder string `json:"responder"`
}

// Validate checks that required fields are present.
func Validate(a *Ask) error {
	if a.To == "" {
		return fmt.Errorf("missing required field: to")
	}
	if a.From == "" {
		return fmt.Errorf("missing required field: from")
	}
	if a.Action == "" {
		return fmt.Errorf("missing required field: action")
	}
	if len(a.Options) == 0 {
		return fmt.Errorf("missing required field: options (need at least one)")
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/ask/ -v -run TestValidate`

Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ask/ask.go internal/ask/ask_test.go
git commit -m "feat: add internal/ask package with types and validation"
```

---

### Task 2: Add GenerateID

**Files:**
- Modify: `internal/ask/ask.go`
- Modify: `internal/ask/ask_test.go`

- [ ] **Step 1: Write failing test for GenerateID**

Append to `internal/ask/ask_test.go`:

```go
import (
	"strings"
	"testing"
)

func TestGenerateID_Format(t *testing.T) {
	id := GenerateID("librarian", "approve-merge")
	if !strings.HasPrefix(id, "ask-") {
		t.Fatalf("expected prefix 'ask-', got: %s", id)
	}
	parts := strings.SplitN(id, "-", 3)
	if len(parts) < 3 {
		t.Fatalf("expected at least 3 parts, got: %s", id)
	}
	// Should contain the from and action
	if !strings.Contains(id, "librarian") {
		t.Fatalf("expected id to contain 'librarian', got: %s", id)
	}
	if !strings.Contains(id, "approve-merge") {
		t.Fatalf("expected id to contain 'approve-merge', got: %s", id)
	}
}

func TestGenerateID_Unique(t *testing.T) {
	id1 := GenerateID("librarian", "approve-merge")
	id2 := GenerateID("librarian", "approve-merge")
	// IDs based on time — same second may collide, but should at least be well-formed
	if !strings.HasPrefix(id1, "ask-") || !strings.HasPrefix(id2, "ask-") {
		t.Fatalf("malformed IDs: %s, %s", id1, id2)
	}
}
```

Note: update the import block at the top of the test file to include `"strings"`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/ask/ -v -run TestGenerateID`

Expected: compilation error — `GenerateID` not defined.

- [ ] **Step 3: Write GenerateID**

Add to `internal/ask/ask.go`:

```go
import (
	"fmt"
	"time"
)

// GenerateID creates a deterministic, sortable ask ID.
// Format: ask-<ISO8601compact>-<from>-<action>
func GenerateID(from, action string) string {
	ts := time.Now().UTC().Format("20060102T150405")
	return fmt.Sprintf("ask-%s-%s-%s", ts, from, action)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/ask/ -v -run TestGenerateID`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ask/ask.go internal/ask/ask_test.go
git commit -m "feat: add ask.GenerateID for deterministic ask IDs"
```

---

### Task 3: Add Write (atomic file delivery)

**Files:**
- Modify: `internal/ask/ask.go`
- Modify: `internal/ask/ask_test.go`

- [ ] **Step 1: Write failing test for Write**

Append to `internal/ask/ask_test.go`:

```go
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite_DeliversToTarget(t *testing.T) {
	tmpDir := t.TempDir()

	// Create target agent directory with asks/pending/
	targetPending := filepath.Join(tmpDir, "agents-home", "alice", "asks", "pending")
	if err := os.MkdirAll(targetPending, 0o755); err != nil {
		t.Fatal(err)
	}

	a := &Ask{
		ID:        "ask-20260402T143000-librarian-approve-merge",
		From:      "librarian",
		To:        "alice",
		Timestamp: "2026-04-02T14:30:00Z",
		Action:    "approve-merge",
		Context: AskContext{
			Reason: "8 overlapping monitors",
		},
		Options:  []string{"approve", "deny"},
		Response: nil,
	}

	if err := Write(tmpDir, a); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(targetPending, a.ID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}

	// Verify content
	var got Ask
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.ID != a.ID {
		t.Fatalf("expected id %q, got %q", a.ID, got.ID)
	}
	if got.From != "librarian" {
		t.Fatalf("expected from 'librarian', got %q", got.From)
	}
}

func TestWrite_CreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't pre-create asks/pending/ — Write should create it
	agentDir := filepath.Join(tmpDir, "agents-home", "alice")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := &Ask{
		ID:      "ask-20260402T143000-librarian-test",
		From:    "librarian",
		To:      "alice",
		Action:  "test",
		Options: []string{"approve"},
	}

	if err := Write(tmpDir, a); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	path := filepath.Join(agentDir, "asks", "pending", a.ID+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}
}

func TestWrite_NoTempFileLeftBehind(t *testing.T) {
	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "agents-home", "alice")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := &Ask{
		ID:      "ask-20260402T143000-librarian-cleanup",
		From:    "librarian",
		To:      "alice",
		Action:  "cleanup",
		Options: []string{"approve"},
	}

	if err := Write(tmpDir, a); err != nil {
		t.Fatal(err)
	}

	pendingDir := filepath.Join(agentDir, "asks", "pending")
	entries, _ := os.ReadDir(pendingDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}
```

Note: update the import block at the top of the test file to include `"encoding/json"`, `"os"`, `"path/filepath"`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/ask/ -v -run TestWrite`

Expected: compilation error — `Write` not defined.

- [ ] **Step 3: Write the Write function**

Add to `internal/ask/ask.go`:

```go
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Write atomically delivers an ask to the target agent's asks/pending/ directory.
func Write(rootDir string, a *Ask) error {
	pendingDir := filepath.Join(rootDir, "agents-home", a.To, "asks", "pending")
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		return fmt.Errorf("creating pending dir: %w", err)
	}

	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling ask: %w", err)
	}
	data = append(data, '\n')

	// Atomic write: temp file → rename
	tmpPath := filepath.Join(pendingDir, ".tmp-"+a.ID+".json")
	finalPath := filepath.Join(pendingDir, a.ID+".json")

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath) // best-effort cleanup
		return fmt.Errorf("renaming to final path: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/ask/ -v -run TestWrite`

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ask/ask.go internal/ask/ask_test.go
git commit -m "feat: add ask.Write with atomic file delivery"
```

---

### Task 4: Add List, ListAll, Count

**Files:**
- Modify: `internal/ask/ask.go`
- Modify: `internal/ask/ask_test.go`

- [ ] **Step 1: Write failing tests for List and Count**

Append to `internal/ask/ask_test.go`:

```go
func writeTestAsk(t *testing.T, dir string, a *Ask) {
	t.Helper()
	pendingDir := filepath.Join(dir, "agents-home", a.To, "asks", "pending")
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(a, "", "  ")
	if err := os.WriteFile(filepath.Join(pendingDir, a.ID+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestAskDone(t *testing.T, dir string, a *Ask) {
	t.Helper()
	doneDir := filepath.Join(dir, "agents-home", a.To, "asks", "done")
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(a, "", "  ")
	if err := os.WriteFile(filepath.Join(doneDir, a.ID+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestList_ReturnsPendingAsks(t *testing.T) {
	tmpDir := t.TempDir()

	writeTestAsk(t, tmpDir, &Ask{
		ID: "ask-20260402T140000-librarian-merge", From: "librarian", To: "alice",
		Timestamp: "2026-04-02T14:00:00Z", Action: "merge", Options: []string{"approve"},
	})
	writeTestAsk(t, tmpDir, &Ask{
		ID: "ask-20260402T150000-librarian-retract", From: "librarian", To: "alice",
		Timestamp: "2026-04-02T15:00:00Z", Action: "retract", Options: []string{"approve"},
	})

	asks, err := List(tmpDir, "alice", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(asks) != 2 {
		t.Fatalf("expected 2 asks, got %d", len(asks))
	}
	// Should be sorted by timestamp ascending
	if asks[0].Timestamp > asks[1].Timestamp {
		t.Fatal("expected asks sorted by timestamp ascending")
	}
}

func TestList_ReturnsDoneAsks(t *testing.T) {
	tmpDir := t.TempDir()

	writeTestAskDone(t, tmpDir, &Ask{
		ID: "ask-20260402T140000-librarian-merge", From: "librarian", To: "alice",
		Timestamp: "2026-04-02T14:00:00Z", Action: "merge", Options: []string{"approve"},
		Response: &AskResponse{Decision: "approve", Reason: "ok"},
	})

	asks, err := List(tmpDir, "alice", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(asks) != 1 {
		t.Fatalf("expected 1 ask, got %d", len(asks))
	}
	if asks[0].Response == nil {
		t.Fatal("expected response to be present")
	}
}

func TestList_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	pendingDir := filepath.Join(tmpDir, "agents-home", "alice", "asks", "pending")
	os.MkdirAll(pendingDir, 0o755)

	asks, err := List(tmpDir, "alice", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(asks) != 0 {
		t.Fatalf("expected 0 asks, got %d", len(asks))
	}
}

func TestList_NonexistentDir(t *testing.T) {
	tmpDir := t.TempDir()

	asks, err := List(tmpDir, "alice", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(asks) != 0 {
		t.Fatalf("expected 0 asks for nonexistent dir, got %d", len(asks))
	}
}

func TestCount_PendingAndDone(t *testing.T) {
	tmpDir := t.TempDir()

	writeTestAsk(t, tmpDir, &Ask{
		ID: "ask-1", From: "librarian", To: "alice", Action: "merge", Options: []string{"approve"},
	})
	writeTestAsk(t, tmpDir, &Ask{
		ID: "ask-2", From: "librarian", To: "alice", Action: "retract", Options: []string{"approve"},
	})
	writeTestAskDone(t, tmpDir, &Ask{
		ID: "ask-3", From: "librarian", To: "alice", Action: "done", Options: []string{"approve"},
	})

	pending, done, err := Count(tmpDir, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if pending != 2 {
		t.Fatalf("expected 2 pending, got %d", pending)
	}
	if done != 1 {
		t.Fatalf("expected 1 done, got %d", done)
	}
}

func TestListAll_MultipleAgents(t *testing.T) {
	tmpDir := t.TempDir()

	writeTestAsk(t, tmpDir, &Ask{
		ID: "ask-1", From: "librarian", To: "alice", Action: "merge", Options: []string{"approve"},
	})
	writeTestAsk(t, tmpDir, &Ask{
		ID: "ask-2", From: "librarian", To: "neo", Action: "retract", Options: []string{"approve"},
	})

	result, err := ListAll(tmpDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result["alice"]) != 1 {
		t.Fatalf("expected 1 ask for alice, got %d", len(result["alice"]))
	}
	if len(result["neo"]) != 1 {
		t.Fatalf("expected 1 ask for neo, got %d", len(result["neo"]))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/ask/ -v -run "TestList|TestCount"`

Expected: compilation error — `List`, `ListAll`, `Count` not defined.

- [ ] **Step 3: Write List, ListAll, Count**

Add to `internal/ask/ask.go`:

```go
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// List reads asks from an agent's pending/ or done/ directory.
// Returns empty slice (not error) if directory doesn't exist.
func List(rootDir, agent string, done bool) ([]Ask, error) {
	subdir := "pending"
	if done {
		subdir = "done"
	}
	dir := filepath.Join(rootDir, "agents-home", agent, "asks", subdir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	var asks []Ask
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var a Ask
		if err := json.Unmarshal(data, &a); err != nil {
			continue
		}
		asks = append(asks, a)
	}

	sort.Slice(asks, func(i, j int) bool {
		return asks[i].Timestamp < asks[j].Timestamp
	})

	return asks, nil
}

// ListAll returns pending or done asks for every agent in agents-home/.
func ListAll(rootDir string, done bool) (map[string][]Ask, error) {
	agentsHome := filepath.Join(rootDir, "agents-home")
	entries, err := os.ReadDir(agentsHome)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading agents-home: %w", err)
	}

	result := make(map[string][]Ask)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		asks, err := List(rootDir, e.Name(), done)
		if err != nil {
			continue
		}
		if len(asks) > 0 {
			result[e.Name()] = asks
		}
	}

	return result, nil
}

// Count returns the number of .json files in pending/ and done/ without parsing.
func Count(rootDir, agent string) (pending, done int, err error) {
	pending, err = countJSON(filepath.Join(rootDir, "agents-home", agent, "asks", "pending"))
	if err != nil {
		return
	}
	done, err = countJSON(filepath.Join(rootDir, "agents-home", agent, "asks", "done"))
	return
}

func countJSON(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			n++
		}
	}
	return n, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/ask/ -v -run "TestList|TestCount"`

Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ask/ask.go internal/ask/ask_test.go
git commit -m "feat: add ask.List, ListAll, Count for reading ask queues"
```

---

### Task 5: Create `cmd/ask.go` — parent command + `ask write`

**Files:**
- Create: `cmd/ask.go`
- Modify: `cmd/root.go:75` — add `rootCmd.AddCommand(askCmd)`

- [ ] **Step 1: Write `cmd/ask.go` with parent and write subcommand**

Create `cmd/ask.go`:

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SeanoChang/cubit/internal/ask"
	"github.com/spf13/cobra"
)

var askCmd = &cobra.Command{
	Use:   "ask",
	Short: "Manage structured inter-agent requests",
}

var askWriteCmd = &cobra.Command{
	Use:   "write",
	Short: "Send a structured ask to another agent's pending queue",
	Long: `Reads a JSON ask from --context-file or stdin, validates it, and delivers
it to the target agent's asks/pending/ directory.

Required JSON fields: to, from, action, options
Optional: id (auto-generated), timestamp (auto-injected), context, response

Example JSON:
  {
    "from": "librarian",
    "to": "alice",
    "action": "approve-merge",
    "context": {"reason": "8 overlapping monitors"},
    "options": ["approve", "deny"]
  }

Usage:
  cubit ask write --context-file proposal.json
  cat proposal.json | cubit ask write`,
	RunE: func(cmd *cobra.Command, args []string) error {
		contextFile, _ := cmd.Flags().GetString("context-file")

		var data []byte
		var err error

		if contextFile != "" {
			// Read from file
			path := contextFile
			if !filepath.IsAbs(path) && agentExplicit {
				path = filepath.Join(cfg.AgentDir(), path)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading context file: %w", err)
			}
		} else {
			// Read from stdin
			info, _ := os.Stdin.Stat()
			if info.Mode()&os.ModeCharDevice != 0 {
				return fmt.Errorf("no input — use --context-file <path> or pipe JSON to stdin")
			}
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
		}

		var a ask.Ask
		if err := json.Unmarshal(data, &a); err != nil {
			return fmt.Errorf("parsing JSON: %w", err)
		}

		// Inject timestamp if missing
		if a.Timestamp == "" {
			a.Timestamp = time.Now().UTC().Format(time.RFC3339)
		}

		// Generate ID if missing
		if a.ID == "" {
			a.ID = ask.GenerateID(a.From, a.Action)
		}

		// Validate required fields
		if err := ask.Validate(&a); err != nil {
			return err
		}

		// Validate agent names
		if !isValidAgentName(a.To) {
			return fmt.Errorf("invalid agent name in 'to': %q", a.To)
		}
		if !isValidAgentName(a.From) {
			return fmt.Errorf("invalid agent name in 'from': %q", a.From)
		}

		// Verify target agent exists
		agentsHome := filepath.Join(cfg.Root, "agents-home")
		targetDir := filepath.Join(agentsHome, a.To)
		if rel, err := filepath.Rel(agentsHome, targetDir); err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("invalid target agent: %q", a.To)
		}
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			return fmt.Errorf("unknown agent: %s (no workspace at %s)", a.To, targetDir)
		}

		// Deliver
		if err := ask.Write(cfg.Root, &a); err != nil {
			return fmt.Errorf("delivering ask: %w", err)
		}

		// Output result as JSON
		result := map[string]string{
			"id":           a.ID,
			"delivered_to": fmt.Sprintf("%s/asks/pending/", a.To),
		}
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	askWriteCmd.Flags().String("context-file", "", "Path to JSON ask file")

	askCmd.AddCommand(askWriteCmd)
}
```

- [ ] **Step 2: Register askCmd in root.go**

In `cmd/root.go`, add after line 75 (`rootCmd.AddCommand(migrateMailboxCmd)`):

```go
rootCmd.AddCommand(askCmd)
```

- [ ] **Step 3: Build and verify compilation**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go build -o cubit .`

Expected: builds without errors.

- [ ] **Step 4: Verify help output**

Run: `./cubit ask --help && ./cubit ask write --help`

Expected: shows help text for ask and ask write commands.

- [ ] **Step 5: Commit**

```bash
git add cmd/ask.go cmd/root.go
git commit -m "feat: add cubit ask write command"
```

---

### Task 6: Add `ask list` subcommand

**Files:**
- Modify: `cmd/ask.go`

- [ ] **Step 1: Add askListCmd to `cmd/ask.go`**

Add before the `init()` function in `cmd/ask.go`:

```go
var askListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List pending or done asks for an agent",
	Long: `Show asks in an agent's queue.

Examples:
  cubit ask list                     # pending asks for current agent
  cubit ask list --done              # done asks for current agent
  cubit ask list --agent alice       # pending asks for alice
  cubit ask list --all               # pending asks for all agents
  cubit ask list --count             # count only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showAll, _ := cmd.Flags().GetBool("all")
		showDone, _ := cmd.Flags().GetBool("done")
		showCount, _ := cmd.Flags().GetBool("count")
		agentName, _ := cmd.Flags().GetString("agent")

		if showAll {
			if showCount {
				// Count across all agents
				agentsHome := filepath.Join(cfg.Root, "agents-home")
				entries, err := os.ReadDir(agentsHome)
				if err != nil {
					if os.IsNotExist(err) {
						fmt.Println("{}")
						return nil
					}
					return fmt.Errorf("reading agents-home: %w", err)
				}
				counts := make(map[string]map[string]int)
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					p, d, err := ask.Count(cfg.Root, e.Name())
					if err != nil {
						continue
					}
					if p > 0 || d > 0 {
						counts[e.Name()] = map[string]int{"pending": p, "done": d}
					}
				}
				out, _ := json.MarshalIndent(counts, "", "  ")
				fmt.Println(string(out))
				return nil
			}

			result, err := ask.ListAll(cfg.Root, showDone)
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		// Single agent
		agent := cfg.Agent
		if agentName != "" {
			agent = agentName
		}
		if agent == "" {
			return fmt.Errorf("agent not specified — use --agent <name> or --all")
		}

		if showCount {
			p, d, err := ask.Count(cfg.Root, agent)
			if err != nil {
				return err
			}
			out, _ := json.Marshal(map[string]int{"pending": p, "done": d})
			fmt.Println(string(out))
			return nil
		}

		asks, err := ask.List(cfg.Root, agent, showDone)
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(asks, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}
```

Update `init()` to register list and its flags:

```go
func init() {
	askWriteCmd.Flags().String("context-file", "", "Path to JSON ask file")

	askListCmd.Flags().String("agent", "", "Target agent name")
	askListCmd.Flags().Bool("all", false, "Show asks for all agents")
	askListCmd.Flags().Bool("done", false, "Show done asks instead of pending")
	askListCmd.Flags().Bool("count", false, "Show counts only")

	askCmd.AddCommand(askWriteCmd)
	askCmd.AddCommand(askListCmd)
}
```

- [ ] **Step 2: Build and verify compilation**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go build -o cubit .`

Expected: builds without errors.

- [ ] **Step 3: Verify help output**

Run: `./cubit ask list --help`

Expected: shows help text with `--agent`, `--all`, `--done`, `--count` flags.

- [ ] **Step 4: Commit**

```bash
git add cmd/ask.go
git commit -m "feat: add cubit ask list command"
```

---

### Task 7: End-to-end manual test

**Files:** None (verification only)

- [ ] **Step 1: Test ask write with a file**

```bash
cd /Users/seanochang/dev/projects/agents/cubit

# Create a test ask JSON
cat > /tmp/test-ask.json << 'EOF'
{
  "from": "librarian",
  "to": "noah",
  "action": "approve-merge",
  "context": {
    "reason": "3 overlapping notes in finance/research",
    "note_ids": ["abc123", "def456"],
    "note_titles": ["Note A", "Note B"],
    "proposed_result": "Merge into consolidated note"
  },
  "options": ["approve", "deny", "modify"]
}
EOF

./cubit ask write --context-file /tmp/test-ask.json
```

Expected: JSON output with `id` and `delivered_to` fields. File created at `~/.ark/agents-home/noah/asks/pending/<id>.json`.

- [ ] **Step 2: Test ask list**

```bash
./cubit ask list --agent noah
```

Expected: JSON array with the ask we just wrote.

- [ ] **Step 3: Test ask list --count**

```bash
./cubit ask list --agent noah --count
```

Expected: `{"pending":1,"done":0}`

- [ ] **Step 4: Test ask write via stdin**

```bash
echo '{"from":"librarian","to":"noah","action":"retract","context":{"reason":"stale note"},"options":["approve","deny"]}' | ./cubit ask write
```

Expected: JSON output confirming delivery.

- [ ] **Step 5: Test ask list --all**

```bash
./cubit ask list --all
```

Expected: JSON object with agent names as keys.

- [ ] **Step 6: Clean up test asks**

```bash
rm ~/.ark/agents-home/noah/asks/pending/ask-*.json 2>/dev/null
```

- [ ] **Step 7: Run all tests**

Run: `cd /Users/seanochang/dev/projects/agents/cubit && go test ./... -v`

Expected: all tests pass, including the new `internal/ask/` tests.
