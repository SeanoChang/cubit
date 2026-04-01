package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var targets = map[string]string{
	"goals":      "GOALS.md",
	"memory":     "MEMORY.md",
	"program":    "PROGRAM.md",
	"fluctlight": "FLUCTLIGHT.md",
	"settings":   filepath.Join(".claude", "settings.json"),
}

const memoryDirPrefix = "memory/"

var editCmd = &cobra.Command{
	Use:   "edit <target> [content]",
	Short: "Edit an agent file — write content directly or open $EDITOR",
	Long: `Targets: goals, memory, program, fluctlight, settings

Agent mode (write directly):
  cubit edit memory "new content here"
  cubit edit goals --file goals-draft.md
  echo "content" | cubit edit memory

Human mode (opens $EDITOR):
  cubit edit memory
  cubit edit goals

Requires --agent <name> when not inside an agent directory.`,
	Args:      cobra.RangeArgs(1, 2),
	ValidArgs: []string{"goals", "memory", "program", "fluctlight", "settings"},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Require explicit agent resolution for edit
		if !agentExplicit {
			return fmt.Errorf("agent not specified — use --agent <name> or run from inside an agent directory")
		}

		target := args[0]
		filename, ok := targets[target]
		if !ok && strings.HasPrefix(target, memoryDirPrefix) {
			// memory/<path> → memory/<path>.md
			sub := strings.TrimPrefix(target, memoryDirPrefix)
			if sub == "" {
				return fmt.Errorf("missing path after memory/ — use: cubit edit memory/<topic>")
			}
			sub = filepath.Clean(sub)
			if strings.HasPrefix(sub, "..") || filepath.IsAbs(sub) {
				return fmt.Errorf("invalid memory path %q — must be relative within memory/", sub)
			}
			if !strings.HasSuffix(sub, ".md") {
				sub += ".md"
			}
			filename = filepath.Join("memory", sub)
			ok = true
		}
		if !ok {
			return fmt.Errorf("unknown target %q — use: goals, memory, memory/<topic>, program, fluctlight, settings", target)
		}

		path := filepath.Join(cfg.AgentDir(), filename)

		// Ensure parent directory exists for nested memory paths
		if strings.HasPrefix(target, memoryDirPrefix) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
		}
		append, _ := cmd.Flags().GetBool("append")

		// Mode 1: content as argument
		if len(args) == 2 {
			return writeContent(path, []byte(args[1]+"\n"), append)
		}

		// Mode 2: content from file
		fromFile, _ := cmd.Flags().GetString("file")
		if fromFile != "" {
			data, err := os.ReadFile(fromFile)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			return writeContent(path, data, append)
		}

		// Mode 3: piped stdin
		if !isTerminal() {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			return writeContent(path, data, append)
		}

		// Mode 4: human mode — open $EDITOR
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

func init() {
	editCmd.Flags().String("file", "", "Write content from a file")
	editCmd.Flags().Bool("append", false, "Append instead of replacing")
}

func writeContent(path string, data []byte, append bool) error {
	if append {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
