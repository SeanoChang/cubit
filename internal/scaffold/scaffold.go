package scaffold

import (
	"os"
	"path/filepath"

	"github.com/SeanoChang/cubit/internal/config"
	"gopkg.in/yaml.v3"
)

// Init creates the agent directory structure under root/agent.
// Returns (true, nil) if created, (false, nil) if already exists.
func Init(root, agent string) (bool, error) {
	agentDir := filepath.Join(root, agent)

	// Already initialized — skip
	if _, err := os.Stat(agentDir); err == nil {
		return false, nil
	}

	dirs := []string{
		filepath.Join(agentDir, "identity"),
		filepath.Join(agentDir, "queue"),
		filepath.Join(agentDir, "scratch"),
		filepath.Join(agentDir, "memory", "sessions"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return false, err
		}
	}

	// Placeholder files — FLUCTLIGHT.md and USER.md are overwritten by onboarding if it runs
	files := map[string]string{
		filepath.Join(agentDir, "identity", "FLUCTLIGHT.md"): "",
		filepath.Join(agentDir, "USER.md"):                   "",
		filepath.Join(agentDir, "GOALS.md"):                  "# Goals\n\n<!-- Agent-managed. Updated as work progresses. -->\n",
		filepath.Join(agentDir, "memory", "brief.md"):        "",
		filepath.Join(agentDir, "memory", "log.md"):          "",
		filepath.Join(agentDir, "state.json"):                "{}",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return false, err
		}
	}

	// Write default config if it doesn't exist
	configPath := filepath.Join(root, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := config.Default(agent)
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return false, err
		}
		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			return false, err
		}
	}

	return true, nil
}
