package exec

import "testing"

func TestSummarize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"first line", "Implemented FTS5 insert.\nMore details here.", "Implemented FTS5 insert."},
		{"long line truncated", string(make([]byte, 300)), string(make([]byte, 200))},
		{"empty", "", "completed"},
		{"whitespace only", "   \n\n  ", "completed"},
		{"single line", "Done.", "Done."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarize(tt.input)
			if got != tt.want {
				t.Errorf("summarize() = %q, want %q", got, tt.want)
			}
		})
	}
}
