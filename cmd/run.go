package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/SeanoChang/cubit/internal/brief"
	"github.com/SeanoChang/cubit/internal/claude"
	"github.com/SeanoChang/cubit/internal/queue"
	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Resolve the task DAG: fan-out ready tasks, fan-in at dependencies",
	Long:  "Concurrent DAG executor. Finds all ready tasks, runs them in parallel (up to --max-parallel), waits for completions to unlock dependents. Stops when graph is fully resolved or deadlocked.",
	RunE: func(cmd *cobra.Command, args []string) error {
		once, _ := cmd.Flags().GetBool("once")
		cooldown, _ := cmd.Flags().GetDuration("cooldown")
		noMemory, _ := cmd.Flags().GetBool("no-memory")
		maxParallel, _ := cmd.Flags().GetInt("max-parallel")

		if maxParallel <= 0 {
			maxParallel = cfg.Claude.MaxParallel
		}
		if maxParallel <= 0 {
			maxParallel = runtime.NumCPU() * 4
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		sem := semaphore.NewWeighted(int64(maxParallel))
		doneCh := make(chan queue.TaskResult, 64)
		running := 0
		dispatched := make(map[int]bool)

		fmt.Printf("Starting DAG executor (max-parallel: %d)\n", maxParallel)

		for {
			// Check for cancellation
			select {
			case <-ctx.Done():
				fmt.Println("\nInterrupted. Waiting for running tasks...")
				for running > 0 {
					result := <-doneCh
					running--
					handleResult(result, noMemory)
				}
				return nil
			default:
			}

			// Scan for ready nodes
			pending, err := q.List()
			if err != nil {
				return err
			}
			active, err := q.Active()
			if err != nil {
				return err
			}
			doneList, err := q.ListDone()
			if err != nil {
				return err
			}

			ready := queue.ReadyNodes(pending, active, doneList)

			// Filter out already-dispatched tasks
			var toDispatch []*queue.Task
			for _, t := range ready {
				if !dispatched[t.ID] {
					toDispatch = append(toDispatch, t)
				}
			}

			// Launch ready tasks
			for range toDispatch {
				popped, err := q.PopReady()
				if err != nil {
					break
				}

				dispatched[popped.ID] = true

				// Acquire a semaphore slot (blocks until one is free, or ctx cancels)
				if err := sem.Acquire(ctx, 1); err != nil {
					if rerr := q.RequeueByID(popped.ID); rerr != nil {
						fmt.Fprintf(os.Stderr, "  requeue error %03d: %v\n", popped.ID, rerr)
					}
					for running > 0 {
						r := <-doneCh
						running--
						handleResult(r, noMemory)
					}
					return nil
				}

				running++
				fmt.Printf("▶ %03d: %s\n", popped.ID, popped.Title)

				go func(t *queue.Task, nm bool) {
					defer sem.Release(1)
					var result queue.TaskResult
					if t.Mode == "loop" {
						result = executeLoop(ctx, t, nm)
					} else {
						result = executeWithRetry(ctx, t, 3)
					}
					doneCh <- result
				}(popped, noMemory)

				if once {
					break
				}
			}

			// Terminal condition
			if running == 0 {
				// Re-scan after dispatching (pending list may have changed)
				pending, _ = q.List()
				active, _ = q.Active()
				doneList, _ = q.ListDone()
				if queue.GraphComplete(pending, active, doneList) {
					fmt.Println("Graph resolved. Done.")
					return nil
				}
				return &queue.DeadlockError{Stuck: pending}
			}

			// Wait for exactly one completion
			result := <-doneCh
			running--
			delete(dispatched, result.TaskID)
			handleResult(result, noMemory)

			if once {
				// Drain remaining and exit
				for running > 0 {
					r := <-doneCh
					running--
					delete(dispatched, r.TaskID)
					handleResult(r, noMemory)
				}
				return nil
			}

			// Cooldown between re-scans
			if cooldown > 0 {
				if !sleepOrCancel(ctx, cooldown) {
					for running > 0 {
						r := <-doneCh
						running--
						handleResult(r, noMemory)
					}
					return nil
				}
			}
		}
	},
}

func executeWithRetry(ctx context.Context, task *queue.Task, maxRetries int) queue.TaskResult {
	agentDir := cfg.AgentDir()
	scratchDir := filepath.Join(agentDir, "scratch")

	// Build brief with upstream output paths for fan-in nodes
	injection := brief.BuildWithUpstream(agentDir, task.DependsOn)
	full := injection + "\n\n---\n\nExecute the active task."

	// Resolve model: task override → config default
	model := task.Model
	if model == "" {
		model = cfg.Claude.Model
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return queue.TaskResult{
				TaskID: task.ID,
				Err:    ctx.Err(),
				Model:  model,
			}
		default:
		}

		if attempt > 0 {
			fmt.Fprintf(os.Stderr, "  %03d: retry %d/%d\n", task.ID, attempt, maxRetries)
		}

		output, err := claude.Prompt(full, model)
		if err != nil {
			lastErr = err
			continue
		}

		// Success
		if err := queue.WriteTaskOutput(scratchDir, task.ID, output); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: write output %03d: %v\n", task.ID, err)
		}
		return queue.TaskResult{
			TaskID:  task.ID,
			Output:  output,
			Summary: summarize(output),
			Model:   model,
		}
	}

	// All retries exhausted
	if err := queue.WriteTaskOutput(scratchDir, task.ID, ""); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: write output %03d: %v\n", task.ID, err)
	}
	return queue.TaskResult{
		TaskID:  task.ID,
		Summary: fmt.Sprintf("FAILED after %d attempts: %v", maxRetries+1, lastErr),
		Err:     lastErr,
		Model:   model,
	}
}

// executeLoop runs a loop-mode task: iterate until goal met, max_iterations, or cancellation.
func executeLoop(ctx context.Context, task *queue.Task, noMemory bool) queue.TaskResult {
	agentDir := cfg.AgentDir()
	scratchDir := filepath.Join(agentDir, "scratch")

	model := task.Model
	if model == "" {
		model = cfg.Claude.Model
	}

	maxIter := task.MaxIterations // 0 = unlimited

	for {
		select {
		case <-ctx.Done():
			return queue.TaskResult{
				TaskID:  task.ID,
				Summary: "interrupted",
				Err:     ctx.Err(),
				Model:   model,
			}
		default:
		}

		iteration := queue.IncrementIteration(scratchDir, task.ID)

		// Check max_iterations
		if maxIter > 0 && iteration > maxIter {
			queue.ClearIteration(scratchDir, task.ID)
			return queue.TaskResult{
				TaskID:  task.ID,
				Summary: fmt.Sprintf("max iterations reached (%d)", maxIter),
				Model:   model,
			}
		}

		fmt.Printf("  ↻ %03d: iteration %d", task.ID, iteration)
		if maxIter > 0 {
			fmt.Printf("/%d", maxIter)
		}
		fmt.Println()

		// Build loop injection
		injection := brief.BuildLoopInjection(agentDir, task.Program, task.Goal, iteration, maxIter)
		full := injection + "\n\n---\n\nExecute the next iteration of the active loop task."

		output, err := claude.Prompt(full, model)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %03d: iteration %d error: %v\n", task.ID, iteration, err)
			continue // retry next iteration on transient errors
		}

		// Write output (overwrite each iteration — latest output wins)
		if writeErr := queue.WriteTaskOutput(scratchDir, task.ID, output); writeErr != nil {
			fmt.Fprintf(os.Stderr, "  warning: write output %03d: %v\n", task.ID, writeErr)
		}

		fmt.Printf("\n%s\n\n", output)

		// Memory pass between iterations
		if !noMemory {
			if memErr := brief.RunMemoryPass(agentDir, output, cfg.Claude.MemoryModel); memErr != nil {
				fmt.Fprintf(os.Stderr, "  warning: memory pass failed: %v\n", memErr)
			}
		}

		// Check goal
		if task.Goal != "" && queue.GoalMet(output) {
			queue.ClearIteration(scratchDir, task.ID)
			return queue.TaskResult{
				TaskID:  task.ID,
				Output:  output,
				Summary: fmt.Sprintf("goal met at iteration %d: %s", iteration, task.Goal),
				Model:   model,
			}
		}
	}
}

func handleResult(result queue.TaskResult, noMemory bool) {
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "✗ %03d: %s\n", result.TaskID, result.Err)
		// Requeue interrupted tasks so they can resume
		if result.Err == context.Canceled || result.Err == context.DeadlineExceeded {
			if err := q.RequeueByID(result.TaskID); err != nil {
				fmt.Fprintf(os.Stderr, "  requeue error %03d: %v\n", result.TaskID, err)
			} else {
				fmt.Printf("  ↩ %03d: requeued\n", result.TaskID)
			}
			return
		}
		if err := q.CompleteByID(result.TaskID, result.Summary); err != nil {
			fmt.Fprintf(os.Stderr, "  complete error %03d: %v\n", result.TaskID, err)
		}
		return
	}

	fmt.Printf("\n%s\n\n", result.Output)

	if err := q.CompleteByID(result.TaskID, result.Summary); err != nil {
		fmt.Fprintf(os.Stderr, "  complete error %03d: %v\n", result.TaskID, err)
		return
	}
	fmt.Printf("✓ %03d\n", result.TaskID)

	if !noMemory {
		if err := brief.RunMemoryPass(cfg.AgentDir(), result.Output, cfg.Claude.MemoryModel); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: memory pass failed: %v\n", err)
		}
	}
}

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
