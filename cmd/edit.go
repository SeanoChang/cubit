package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var targets = map[string]string{
	"goals":      "GOALS.md",
	"memory":     "MEMORY.md",
	"program":    "PROGRAM.md",
	"fluctlight": "FLUCTLIGHT.md",
	"settings":   filepath.Join(".claude", "settings.json"),
}

var editCmd = &cobra.Command{
	Use:       "edit <target>",
	Short:     "Open an agent file in $EDITOR",
	Long:      "Targets: goals, memory, program, fluctlight, settings",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"goals", "memory", "program", "fluctlight", "settings"},
	RunE: func(cmd *cobra.Command, args []string) error {
		filename, ok := targets[args[0]]
		if !ok {
			return fmt.Errorf("unknown target %q — use: goals, memory, program, fluctlight, settings", args[0])
		}

		path := filepath.Join(cfg.AgentDir(), filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("%s not found at %s", filename, path)
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		c := exec.Command(editor, path)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}
