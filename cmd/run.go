package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/SeanoChang/cubit/internal/claude"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Drain the task queue: pop → prompt → done → next",
	Long:  "Resolve the linear task queue end-to-end. Pops the next task, sends it to Claude with brief injection, completes it, runs a memory pass, and moves to the next task. Stops when the queue is empty.",
	RunE: func(cmd *cobra.Command, args []string) error {
		once, _ := cmd.Flags().GetBool("once")
		cooldown, _ := cmd.Flags().GetDuration("cooldown")

		// Graceful shutdown on SIGINT.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		for {
			// Check for active task first (resume interrupted run).
			task, err := q.Active()
			if err != nil {
				return err
			}

			// No active task — pop next.
			if task == nil {
				task, err = q.Pop()
				if err != nil {
					fmt.Println("Queue empty. Done.")
					return nil
				}
			}

			fmt.Printf("▶ %03d: %s\n", task.ID, task.Title)

			// Build brief (includes .doing) + canned message.
			injection := brief.Build(cfg.AgentDir())
			full := injection + "\n\n---\n\nExecute the active task."

			result, err := claude.Prompt(full, cfg.Claude.Model)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  error: %v — requeuing\n", err)
				if reqErr := q.Requeue(); reqErr != nil {
					fmt.Fprintf(os.Stderr, "  requeue failed: %v\n", reqErr)
				}
				if once {
					return nil
				}
				if !sleepOrCancel(ctx, cooldown) {
					return nil
				}
				continue
			}

			fmt.Printf("\n%s\n\n", result)

			// Complete the task with auto-summary.
			if err := q.Complete(summarize(result)); err != nil {
				return fmt.Errorf("completing task: %w", err)
			}
			fmt.Printf("✓ %03d: %s\n", task.ID, task.Title)

			// Memory pass (non-fatal).
			if err := brief.RunMemoryPass(cfg.AgentDir(), result, cfg.Claude.MemoryModel); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: memory pass failed: %v\n", err)
			}

			if once {
				return nil
			}

			// Cooldown between tasks, interruptible.
			if !sleepOrCancel(ctx, cooldown) {
				return nil
			}
		}
	},
}

// sleepOrCancel waits for the duration or until ctx is cancelled.
// Returns true if the sleep completed, false if interrupted.
func sleepOrCancel(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		fmt.Println("\nInterrupted. Shutting down.")
		return false
	}
}
