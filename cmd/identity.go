package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Manage agent identity files (FLUCTLIGHT.md, USER.md, GOALS.md, etc.)",
}

var identityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List identity files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := filepath.Join(cfg.AgentDir(), "identity")
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("identity directory not found: %s", dir)
			}
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				fmt.Println(e.Name())
			}
		}
		return nil
	},
}

var identityShowCmd = &cobra.Command{
	Use:   "show <filename>",
	Short: "Print an identity file to stdout",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := validateFilename(name); err != nil {
			return err
		}
		path := filepath.Join(cfg.AgentDir(), "identity", name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("identity file not found: %s", name)
			}
			return err
		}
		fmt.Print(string(data))
		return nil
	},
}

var identitySetCmd = &cobra.Command{
	Use:   "set <filename> [-f path]",
	Short: "Replace an identity file from a local file or stdin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := validateFilename(name); err != nil {
			return err
		}

		filePath, _ := cmd.Flags().GetString("file")

		var content []byte
		var err error

		if filePath != "" {
			content, err = os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read source file: %w", err)
			}
		} else {
			// Check if stdin is piped
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) != 0 {
				return fmt.Errorf("no input: use -f <path> or pipe content via stdin")
			}
			content, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
		}

		dest := filepath.Join(cfg.AgentDir(), "identity", name)

		// Ensure identity directory exists
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}

		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return err
		}

		fmt.Printf("wrote %s\n", dest)
		return nil
	},
}

// validateFilename rejects path traversal attempts.
func validateFilename(name string) error {
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || name == ".." || name == "." {
		return fmt.Errorf("invalid filename: %q (must be a plain filename, no path separators)", name)
	}
	return nil
}
