package mcp

import (
	"os"
	"strings"
)

// cleanEnv returns the current environment with CLAUDE_* variables stripped.
// This prevents spawned processes from thinking they're inside Claude Code.
func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		if strings.HasPrefix(key, "CLAUDE_") || strings.HasPrefix(key, "CLAUDE ") {
			continue
		}
		env = append(env, e)
	}
	return env
}
