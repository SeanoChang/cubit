package cmd

import "strings"

// isValidAgentName checks that an agent name is safe for path construction.
// Rejects empty, ".", "..", names with path separators, or embedded "..".
func isValidAgentName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return false
	}
	return true
}
