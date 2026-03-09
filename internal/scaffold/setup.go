package scaffold

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/SeanoChang/cubit/internal/claude"
)

var execCommand = exec.Command

var errInterrupted = errors.New("interrupted")

const maxRounds = 10

const onboardingPrompt = `You are setting up a new agent named %q. Learn about:

For the agent (FLUCTLIGHT.md):
- Purpose and core role
- Communication style and constraints
- What it should never do

For the human (USER.md):
- Technical background
- Language and communication preferences
- Trust level (how much autonomy the agent gets)

Ask one question at a time. Be conversational, not a form. When you have enough to write both files, you MUST respond with ONLY the text [READY] on its own line and nothing else. Do not say goodbye, do not summarize — just [READY].

%s`

const generateFluctlightPrompt = `Based on this conversation, write FLUCTLIGHT.md for agent %q.

Follow this structure:
- # FLUCTLIGHT — The Soul of {name}
- ## Identity (who it is, one paragraph)
- ## Purpose (what it does, bullet points)
- ## Principles (numbered rules)
- ## Communication (language, style, constraints)

Output ONLY the markdown, no commentary.

%s`

const generateUserPrompt = `Based on this conversation, write USER.md.

Follow this structure:
- # User
- ## Background (technical skills, role)
- ## Language (spoken/written language preferences)
- ## Communication (style preferences)
- ## Trust Level (how much autonomy the agent gets)

Output ONLY the markdown, no commentary.

%s`

// RunSetup runs the interactive onboarding chat, then generates FLUCTLIGHT.md and USER.md.
func RunSetup(agentDir, agent string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	scanner := bufio.NewScanner(os.Stdin)
	var history []string

	fmt.Println("\nStarting agent setup — answer a few questions to configure the workspace.")

	for range maxRounds {
		if err := ctx.Err(); err != nil {
			fmt.Println("\nSetup interrupted. Run `cubit init --force` to retry.")
			return nil
		}

		conversationStr := strings.Join(history, "\n")
		prompt := fmt.Sprintf(onboardingPrompt, agent, conversationStr)

		fmt.Print("Thinking...")
		response, err := claude.Prompt(prompt)
		fmt.Print("\r            \r") // clear "Thinking..."
		if err != nil {
			if ctx.Err() != nil {
				fmt.Println("\nSetup interrupted. Run `cubit init --force` to retry.")
				return nil
			}
			return fmt.Errorf("onboarding chat: %w", err)
		}

		if isReady(response) {
			break
		}

		fmt.Printf("\n%s\n\n> ", response)

		if !scanner.Scan() {
			if ctx.Err() != nil {
				fmt.Println("\nSetup interrupted. Run `cubit init --force` to retry.")
				return nil
			}
			break
		}
		answer := strings.TrimSpace(scanner.Text())
		if answer == "" {
			continue
		}
		if isUserDone(answer) {
			history = append(history, "Assistant: "+response, "User: "+answer)
			break
		}

		history = append(history, "Assistant: "+response, "User: "+answer)
	}

	if ctx.Err() != nil {
		return nil
	}

	conversationStr := strings.Join(history, "\n")

	// Generate and confirm FLUCTLIGHT.md
	fmt.Println("\nGenerating agent identity...")
	flPath := filepath.Join(agentDir, "identity", "FLUCTLIGHT.md")
	if err := generateAndConfirm(ctx, scanner, flPath, fmt.Sprintf(generateFluctlightPrompt, agent, conversationStr)); err != nil {
		if errors.Is(err, errInterrupted) {
			fmt.Println("\nSetup interrupted. Run `cubit init --force` to retry.")
			return nil
		}
		return fmt.Errorf("FLUCTLIGHT.md: %w", err)
	}

	// Generate and confirm USER.md
	fmt.Println("\nGenerating user profile...")
	userPath := filepath.Join(agentDir, "USER.md")
	if err := generateAndConfirm(ctx, scanner, userPath, fmt.Sprintf(generateUserPrompt, conversationStr)); err != nil {
		if errors.Is(err, errInterrupted) {
			fmt.Println("\nSetup interrupted. Run `cubit init --force` to retry.")
			return nil
		}
		return fmt.Errorf("USER.md: %w", err)
	}

	fmt.Println("\nSetup complete.")
	return nil
}

// generateAndConfirm generates content via LLM, previews it, and asks for confirmation.
// Regenerates on "n", opens $EDITOR on "edit".
func generateAndConfirm(ctx context.Context, scanner *bufio.Scanner, path, prompt string) error {
	for {
		if ctx.Err() != nil {
			return errInterrupted
		}

		content, err := claude.Prompt(prompt)
		if err != nil {
			if ctx.Err() != nil {
				return errInterrupted
			}
			return err
		}

		fmt.Printf("\n%s\n", preview(content))
		fmt.Print("Save? (y/n/edit) ")

		if !scanner.Scan() {
			if ctx.Err() != nil {
				return errInterrupted
			}
			return fmt.Errorf("unexpected end of input")
		}

		switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
		case "y", "yes", "":
			if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
				return err
			}
			fmt.Printf("  wrote %s\n", path)
			return nil
		case "n", "no":
			fmt.Println("Regenerating...")
			continue
		case "edit":
			if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
				return err
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			cmd := execCommand(editor, path)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			fmt.Printf("  wrote %s\n", path)
			return nil
		default:
			fmt.Print("Save? (y/n/edit) ")
		}
	}
}

func preview(content string) string {
	bar := strings.Repeat("─", 60)
	return fmt.Sprintf("%s\n%s\n%s", bar, content, bar)
}

// isReady checks if the model's response signals it has enough info.
func isReady(response string) bool {
	return strings.Contains(response, "[READY]")
}

// isUserDone checks if the user wants to end the conversation and move to generation.
func isUserDone(answer string) bool {
	lower := strings.ToLower(strings.TrimSpace(answer))
	switch lower {
	case "done", "that's it", "thats it", "go", "ready", "generate", "next":
		return true
	}
	return false
}
