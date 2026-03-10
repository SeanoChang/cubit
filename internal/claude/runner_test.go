package claude

import (
	"testing"
)

func TestBuildArgs(t *testing.T) {
	tests := []struct {
		name string
		opts RunnerOpts
		want []string
	}{
		{
			name: "minimal",
			opts: RunnerOpts{},
			want: []string{"-p"},
		},
		{
			name: "model only",
			opts: RunnerOpts{Model: "claude-sonnet-4-6"},
			want: []string{"-p", "--model", "claude-sonnet-4-6"},
		},
		{
			name: "all fields",
			opts: RunnerOpts{
				Model:          "claude-opus-4-6",
				PermissionMode: "dontAsk",
				AllowedTools:   []string{"Bash", "Read", "Write"},
			},
			want: []string{"-p", "--model", "claude-opus-4-6", "--permission-mode", "dontAsk", "--allowedTools", "Bash,Read,Write"},
		},
		{
			name: "permission mode only",
			opts: RunnerOpts{PermissionMode: "dontAsk"},
			want: []string{"-p", "--permission-mode", "dontAsk"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildArgs(tt.opts)
			if len(got) != len(tt.want) {
				t.Fatalf("buildArgs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
