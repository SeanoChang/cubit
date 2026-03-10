package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Show agent's durable notes (memory/MEMORY.md)",
	Long:  "Freeform durable memory the agent owns and organizes. Survives brief compaction.",
	Args:  cobra.NoArgs,
	RunE:  memoryShowRun,
}

var memoryShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print memory/MEMORY.md",
	Args:  cobra.NoArgs,
	RunE:  memoryShowRun,
}

var memoryAppendCmd = &cobra.Command{
	Use:   "append [content]",
	Short: "Append to memory/MEMORY.md",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := notesPath()
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			return fmt.Errorf("opening notes: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(args[0] + "\n"); err != nil {
			return fmt.Errorf("appending notes: %w", err)
		}
		fmt.Println("appended to memory/MEMORY.md")
		return nil
	},
}

var memoryEditCmd = &cobra.Command{
	Use:   "edit [content]",
	Short: "Overwrite memory/MEMORY.md",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := notesPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(args[0]+"\n"), 0o644); err != nil {
			return fmt.Errorf("writing notes: %w", err)
		}
		fmt.Println("wrote memory/MEMORY.md")
		return nil
	},
}

func memoryShowRun(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(notesPath())
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("(empty — no notes yet)")
			return nil
		}
		return err
	}
	if len(data) == 0 {
		fmt.Println("(empty — no notes yet)")
		return nil
	}
	fmt.Print(string(data))
	return nil
}

func notesPath() string {
	return filepath.Join(getCfg().AgentDir(), "memory", "MEMORY.md")
}
