package queue

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Task represents a single task in the queue.
type Task struct {
	ID            int       `yaml:"id"`
	Status        string    `yaml:"status"`
	Created       time.Time `yaml:"created"`
	Mode          string    `yaml:"mode,omitempty"`
	Model         string    `yaml:"model,omitempty"`
	DependsOn     []int     `yaml:"depends_on,omitempty"`
	Program       string    `yaml:"program,omitempty"`
	Goal          string    `yaml:"goal,omitempty"`
	MaxIterations int       `yaml:"max_iterations,omitempty"`
	Branch        string    `yaml:"branch,omitempty"`
	Title         string    `yaml:"-"` // extracted from body
	Body          string    `yaml:"-"` // markdown body after frontmatter
}

// ParseTask parses a task file (YAML frontmatter + markdown body).
func ParseTask(data []byte) (*Task, error) {
	parts := bytes.SplitN(data, []byte("---"), 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("missing YAML frontmatter delimiters")
	}

	var task Task
	if err := yaml.Unmarshal(parts[1], &task); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	if task.Mode == "" {
		task.Mode = "once"
	}

	task.Body = strings.TrimSpace(string(parts[2]))
	task.Title = extractTitle(task.Body)
	return &task, nil
}

// Serialize writes the task as YAML frontmatter + markdown body.
func (t *Task) Serialize() []byte {
	fm, _ := yaml.Marshal(t)
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n\n")
	buf.WriteString(t.Body)
	buf.WriteString("\n")
	return buf.Bytes()
}

// extractTitle pulls the first markdown heading from the body.
func extractTitle(body string) string {
	var fallback string
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if title, ok := strings.CutPrefix(line, "# "); ok {
			return title
		}
		if fallback == "" && line != "" {
			fallback = line
		}
	}
	return fallback
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a title to a filename-safe slug (max 5 words).
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	words := strings.SplitN(s, "-", 7)
	if len(words) > 5 {
		words = words[:5]
	}
	return strings.Join(words, "-")
}
