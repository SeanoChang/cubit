package cmd

import (
	"fmt"
	"strings"

	"github.com/SeanoChang/cubit/internal/project"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage agent projects",
}

var projectNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new project with git init",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		agentDir := cfg.AgentDir()

		path, err := project.New(agentDir, name)
		if err != nil {
			return err
		}

		fmt.Printf("Created project %q at %s\n", name, path)
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		agentDir := cfg.AgentDir()

		projects, err := project.List(agentDir)
		if err != nil {
			return err
		}

		if len(projects) == 0 {
			fmt.Println("No projects.")
			return nil
		}

		fmt.Printf("Projects (%d):\n", len(projects))
		for _, p := range projects {
			age := project.FormatAge(p.LastCommit)
			extra := ""
			if p.HasEval {
				extra = " [EVAL]"
			}
			fmt.Printf("  %-25s %3d commits, last: %-8s%s\n", p.Name, p.CommitCount, age, extra)
		}

		return nil
	},
}

var projectSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across all project repos",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		agentDir := cfg.AgentDir()

		results, err := project.Search(agentDir, query)
		if err != nil {
			return err
		}

		if len(results) == 0 {
			fmt.Printf("No projects matching %q.\n", query)
			return nil
		}

		fmt.Printf("Found %d project(s) matching %q:\n\n", len(results), query)
		for _, r := range results {
			fmt.Printf("  %s\n", r.Project)
			fmt.Printf("    path: %s\n", r.Path)
			fmt.Printf("    matches: %s\n\n", strings.Join(r.Matches, ", "))
		}

		return nil
	},
}

var projectArchiveCmd = &cobra.Command{
	Use:   "archive <name>",
	Short: "Archive a project to nark",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return project.Archive(cfg.AgentDir(), args[0])
	},
}

var projectStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show detailed project status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		info, log, err := project.Status(cfg.AgentDir(), args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Project: %s\n", info.Name)
		fmt.Printf("Path:    %s\n", info.Path)
		fmt.Printf("Branch:  %s\n", info.Branch)
		fmt.Printf("Commits: %d (last: %s)\n", info.CommitCount, project.FormatAge(info.LastCommit))
		if info.HasEval {
			fmt.Println("Eval:    EVAL.md present")
		}
		if log != "" {
			fmt.Printf("\nRecent commits:\n")
			for _, line := range strings.Split(log, "\n") {
				fmt.Printf("  %s\n", line)
			}
		}

		return nil
	},
}

func init() {
	projectCmd.AddCommand(projectNewCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectSearchCmd)
	projectCmd.AddCommand(projectArchiveCmd)
	projectCmd.AddCommand(projectStatusCmd)
}
