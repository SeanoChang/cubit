package cmd

import (
	"testing"
)

func TestParseDreamOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantPaths []string
	}{
		{
			name: "normal multi-file output",
			input: `FILE: MEMORY.md
# Memory Index
- [Architecture](memory/architecture.md) — system design

FILE: memory/architecture.md
# Architecture
Details here.
`,
			wantLen:   2,
			wantPaths: []string{"MEMORY.md", "memory/architecture.md"},
		},
		{
			name: "path traversal blocked",
			input: `FILE: MEMORY.md
# Index

FILE: memory/../../etc/passwd
bad content

FILE: ../secret.md
more bad content
`,
			wantLen:   1,
			wantPaths: []string{"MEMORY.md"},
		},
		{
			name: "absolute path blocked",
			input: `FILE: MEMORY.md
# Index

FILE: /etc/passwd
bad content
`,
			wantLen:   1,
			wantPaths: []string{"MEMORY.md"},
		},
		{
			name: "memory/archive/ write blocked",
			input: `FILE: MEMORY.md
# Index

FILE: memory/archive/sneaky.md
overwrite archive
`,
			wantLen:   1,
			wantPaths: []string{"MEMORY.md"},
		},
		{
			name: "non-memory path blocked",
			input: `FILE: MEMORY.md
# Index

FILE: GOALS.md
overwrite goals

FILE: .claude/settings.json
overwrite settings
`,
			wantLen:   1,
			wantPaths: []string{"MEMORY.md"},
		},
		{
			name:      "empty output",
			input:     "",
			wantLen:   0,
			wantPaths: nil,
		},
		{
			name: "no MEMORY.md in output",
			input: `FILE: memory/topic.md
some content
`,
			wantLen:   1,
			wantPaths: []string{"memory/topic.md"},
		},
		{
			name: "nested memory path allowed",
			input: `FILE: MEMORY.md
# Index

FILE: memory/projects/trading-system.md
# Trading System
Details.
`,
			wantLen:   2,
			wantPaths: []string{"MEMORY.md", "memory/projects/trading-system.md"},
		},
		{
			name: "preamble text before first FILE marker ignored",
			input: `Here is the consolidated memory:

FILE: MEMORY.md
# Index
`,
			wantLen:   1,
			wantPaths: []string{"MEMORY.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := parseDreamOutput(tt.input)

			if len(files) != tt.wantLen {
				t.Fatalf("got %d files, want %d", len(files), tt.wantLen)
			}

			for i, wantPath := range tt.wantPaths {
				if files[i].path != wantPath {
					t.Errorf("file[%d].path = %q, want %q", i, files[i].path, wantPath)
				}
			}
		})
	}
}

func TestParseDreamOutputContent(t *testing.T) {
	input := `FILE: MEMORY.md
# Memory Index
- [Topic](memory/topic.md) — description

FILE: memory/topic.md
# Topic
Line 1.
Line 2.
`

	files := parseDreamOutput(input)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}

	// Check MEMORY.md content
	if files[0].path != "MEMORY.md" {
		t.Errorf("file[0].path = %q, want MEMORY.md", files[0].path)
	}
	wantIndex := "# Memory Index\n- [Topic](memory/topic.md) — description\n"
	if files[0].content != wantIndex {
		t.Errorf("file[0].content = %q, want %q", files[0].content, wantIndex)
	}

	// Check topic file content
	if files[1].path != "memory/topic.md" {
		t.Errorf("file[1].path = %q, want memory/topic.md", files[1].path)
	}
	wantTopic := "# Topic\nLine 1.\nLine 2.\n"
	if files[1].content != wantTopic {
		t.Errorf("file[1].content = %q, want %q", files[1].content, wantTopic)
	}
}
