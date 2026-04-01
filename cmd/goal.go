package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var validGoalTypes = []string{"session", "background", "consolidation"}

var goalCmd = &cobra.Command{
	Use:   "goal",
	Short: "Manage typed goals in GOALS.md",
}

var goalAddCmd = &cobra.Command{
	Use:   "add <message>",
	Short: "Add a typed goal to GOALS.md",
	Long: `Add a goal with an optional type tag.

This command creates goals with a "from" source prefix — use it for
human-submitted or tool-submitted goals. Agents create "scheduled:" and
"self-directed:" goals by writing to GOALS.md directly.

Types:
  session        (default) Full context, deep work
  background     Fire-and-forget, quick tasks
  consolidation  Memory/maintenance tasks

Examples:
  cubit goal add "research market trends"
  cubit goal add --type background "check disk usage"
  cubit goal add --type consolidation "reorganize memory"
  cubit goal add --from alice "review PR #42"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !agentExplicit {
			return fmt.Errorf("agent not specified — use --agent <name> or run from inside an agent directory")
		}

		message := args[0]
		goalType, _ := cmd.Flags().GetString("type")
		from, _ := cmd.Flags().GetString("from")

		// Validate type
		valid := false
		for _, t := range validGoalTypes {
			if goalType == t {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid type %q — use: session, background, consolidation", goalType)
		}

		goalsPath := filepath.Join(cfg.AgentDir(), "GOALS.md")
		f, err := os.OpenFile(goalsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("opening GOALS.md: %w", err)
		}
		defer f.Close()

		ts := time.Now().Format("2006-01-02 15:04")

		// Build header: ## [timestamp] from <user> [type]
		// Session type is default — no tag needed
		var header string
		if goalType == "session" {
			header = fmt.Sprintf("\n## [%s] from %s\n", ts, from)
		} else {
			header = fmt.Sprintf("\n## [%s] from %s [%s]\n", ts, from, goalType)
		}

		if _, err := fmt.Fprintf(f, "%s%s\n", header, message); err != nil {
			return fmt.Errorf("writing goal: %w", err)
		}

		typeLabel := goalType
		if goalType == "session" {
			typeLabel = "session (default)"
		}
		fmt.Printf("Added %s goal for %s: %s\n", typeLabel, cfg.Agent, message)
		return nil
	},
}

// goalHeader matches: ## [timestamp] source [type]
var goalHeaderRe = regexp.MustCompile(`^## \[([^\]]+)\]\s+(.+)$`)

// goalTypeRe matches a [type] tag at the end of a header
var goalTypeRe = regexp.MustCompile(`\[(\w+)\]\s*$`)

type parsedGoal struct {
	timestamp string
	source    string
	goalType  string
	body      string
}

func parseGoals(content string) []parsedGoal {
	lines := strings.Split(content, "\n")
	var goals []parsedGoal
	var current *parsedGoal

	for _, line := range lines {
		if m := goalHeaderRe.FindStringSubmatch(line); m != nil {
			// Save previous goal
			if current != nil {
				current.body = strings.TrimSpace(current.body)
				goals = append(goals, *current)
			}

			ts := m[1]
			rest := m[2]
			goalType := "session"

			// Check for [type] tag at end
			if tm := goalTypeRe.FindStringSubmatch(rest); tm != nil {
				goalType = tm[1]
				rest = strings.TrimSpace(goalTypeRe.ReplaceAllString(rest, ""))
			}

			current = &parsedGoal{
				timestamp: ts,
				source:    rest,
				goalType:  goalType,
			}
			continue
		}

		if current != nil {
			current.body += line + "\n"
		}
	}

	// Save last goal
	if current != nil {
		current.body = strings.TrimSpace(current.body)
		goals = append(goals, *current)
	}

	return goals
}

var goalLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List current goals with type tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !agentExplicit {
			return fmt.Errorf("agent not specified — use --agent <name> or run from inside an agent directory")
		}

		goalsPath := filepath.Join(cfg.AgentDir(), "GOALS.md")
		data, err := os.ReadFile(goalsPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No GOALS.md found.")
				return nil
			}
			return fmt.Errorf("reading GOALS.md: %w", err)
		}

		filterType, _ := cmd.Flags().GetString("type")

		goals := parseGoals(string(data))
		if len(goals) == 0 {
			fmt.Println("No goals.")
			return nil
		}

		for i, g := range goals {
			if filterType != "" && g.goalType != filterType {
				continue
			}

			typeTag := ""
			if g.goalType != "session" {
				typeTag = fmt.Sprintf(" [%s]", g.goalType)
			}

			preview := g.body
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			preview = strings.ReplaceAll(preview, "\n", " ")

			fmt.Printf("%d. [%s] %s%s\n", i+1, g.timestamp, g.source, typeTag)
			if preview != "" {
				fmt.Printf("   %s\n", preview)
			}
		}

		return nil
	},
}

func init() {
	goalAddCmd.Flags().String("type", "session", "Goal type: session, background, consolidation")
	goalAddCmd.Flags().String("from", "cubit", "Source/author of the goal")

	goalLsCmd.Flags().String("type", "", "Filter by goal type")

	goalCmd.AddCommand(goalAddCmd)
	goalCmd.AddCommand(goalLsCmd)
}
