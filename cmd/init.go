package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SeanoChang/cubit/internal/scaffold"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [agent_name]",
	Short: "Scaffold a new agent workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var agent string
		if len(args) > 0 {
			agent = args[0]
		} else {
			fmt.Print("Agent name: ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				agent = strings.TrimSpace(scanner.Text())
			}
			if agent == "" {
				return fmt.Errorf("agent name is required")
			}
		}

		root := cfg.Root
		agentDir := filepath.Join(root, "agents-home", agent)
		force, _ := cmd.Flags().GetBool("force")

		created, err := scaffold.Init(root, agent, force)
		if err != nil {
			return fmt.Errorf("initializing agent: %w", err)
		}

		if !created {
			fmt.Printf("Agent %q already exists at %s (use --force to re-initialize)\n", agent, agentDir)
			return nil
		}

		if force {
			fmt.Printf("Re-initialized agent %q at %s\n", agent, agentDir)
		} else {
			fmt.Printf("Initialized agent %q at %s\n", agent, agentDir)
		}

		// --import-identity FILE: copy an existing FLUCTLIGHT.md
		importPath, _ := cmd.Flags().GetString("import-identity")
		if importPath != "" {
			data, err := os.ReadFile(importPath)
			if err != nil {
				return fmt.Errorf("reading identity file: %w", err)
			}
			dest := filepath.Join(agentDir, "FLUCTLIGHT.md")
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return err
			}
			fmt.Printf("  imported %s → FLUCTLIGHT.md\n", importPath)
		}

		// Interactive setup via claude -p
		skipSetup, _ := cmd.Flags().GetBool("skip-setup")
		if !skipSetup && isTerminal() {
			if err := interactiveSetup(agentDir, agent, importPath != ""); err != nil {
				fmt.Fprintf(os.Stderr, "Interactive setup error: %v\n", err)
				fmt.Fprintln(os.Stderr, "You can edit the files manually.")
			}
		}

		return nil
	},
}

func init() {
	initCmd.Flags().String("import-identity", "", "Import an existing FLUCTLIGHT.md file")
	initCmd.Flags().Bool("force", false, "Re-initialize an existing agent workspace")
	initCmd.Flags().Bool("skip-setup", false, "Skip interactive setup with Claude")
}

const maxInterviewRounds = 5

type setupTarget struct {
	file    string
	purpose string
}

var setupTargets = []setupTarget{
	{"FLUCTLIGHT.md", "the agent's identity — their role, personality, expertise, and communication style"},
	{"MEMORY.md", "initial working context — project background, tools, constraints, and current state"},
}

func interactiveSetup(agentDir, agent string, identityImported bool) error {
	if _, err := exec.LookPath("claude"); err != nil {
		fmt.Println("\nclaude CLI not found — skipping interactive setup.")
		fmt.Println("Edit FLUCTLIGHT.md and MEMORY.md manually.")
		return nil
	}

	fmt.Println("\n--- Interactive Setup (powered by Claude) ---")
	fmt.Println("Claude will interview you to build each file.")
	fmt.Println("Type !skip to skip a file, !skip-all to skip remaining.\n")

	targets := setupTargets
	if identityImported {
		targets = targets[1:] // skip FLUCTLIGHT.md
	}

	scanner := bufio.NewScanner(os.Stdin)

	for _, t := range targets {
		fmt.Printf("=== %s ===\n\n", t.file)

		var conversation []string // alternating: question, answer, question, answer...
		skipped := false

		for round := 0; round < maxInterviewRounds; round++ {
			// Ask claude for the next question (or DONE if enough context)
			prompt := buildInterviewPrompt(agent, t, conversation, round)

			claude := exec.Command("claude", "-p", prompt)
			claude.Dir = agentDir
			out, err := claude.Output()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  claude error: %v — skipping %s\n", err, t.file)
				skipped = true
				break
			}

			result := strings.TrimSpace(string(out))

			// Claude says it has enough info
			if result == "DONE" {
				break
			}

			// Show question, get user's answer
			fmt.Printf("  %s\n> ", result)
			if !scanner.Scan() {
				break
			}
			answer := strings.TrimSpace(scanner.Text())

			if answer == "!skip" {
				fmt.Printf("  skipped %s\n\n", t.file)
				skipped = true
				break
			}
			if answer == "!skip-all" {
				fmt.Println("  skipping remaining setup")
				return nil
			}

			conversation = append(conversation, result, answer)
		}

		if skipped || len(conversation) == 0 {
			continue
		}

		// Generate the file from the full conversation
		fmt.Printf("  generating %s...\n", t.file)

		genPrompt := buildGeneratePrompt(agent, t, conversation)
		claude := exec.Command("claude", "-p", genPrompt)
		claude.Dir = agentDir
		out, err := claude.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  generation error: %v\n", err)
			continue
		}

		filePath := filepath.Join(agentDir, t.file)
		if err := os.WriteFile(filePath, out, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", t.file, err)
		}
		fmt.Printf("  %s updated\n\n", t.file)
	}

	fmt.Printf("Agent %q is ready at %s\n", agent, filepath.Join(agentDir))
	return nil
}

func buildInterviewPrompt(agent string, t setupTarget, conversation []string, round int) string {
	if round == 0 {
		return fmt.Sprintf(
			"You are helping set up an AI agent workspace for an agent named %q. "+
				"You need to gather information about %s. "+
				"Ask ONE focused question to start understanding this. "+
				"Output ONLY the question text, nothing else.",
			agent, t.purpose)
	}

	convo := formatConversation(conversation)
	return fmt.Sprintf(
		"You are helping set up an AI agent named %q, gathering information about %s.\n\n"+
			"Interview so far:\n%s\n"+
			"If you have enough information to write a good %s, output exactly the word DONE (nothing else).\n"+
			"Otherwise, ask ONE focused follow-up question. Output ONLY the question or DONE.",
		agent, t.purpose, convo, t.file)
}

func buildGeneratePrompt(agent string, t setupTarget, conversation []string) string {
	convo := formatConversation(conversation)
	return fmt.Sprintf(
		"Generate a %s file for an AI agent named %q based on this interview:\n\n"+
			"%s\n"+
			"Create a well-structured markdown document that captures all the information gathered. "+
			"Output ONLY the raw markdown content — no code fences, no preamble, no explanations.",
		t.file, agent, convo)
}

func formatConversation(conversation []string) string {
	var b strings.Builder
	for i := 0; i < len(conversation)-1; i += 2 {
		fmt.Fprintf(&b, "Q: %s\nA: %s\n\n", conversation[i], conversation[i+1])
	}
	return b.String()
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
