# v0.4 — Loop Execution + Memory

**Milestones:** M10, M11, M12
**Status:** In Progress

## M10: Loop Execution (partially shipped)

- `mode: loop` tasks re-execute until goal met or `max_iterations` reached
- Program.md re-injection each iteration
- `GOAL_MET` agent-driven goal evaluation
- `results.tsv` append-only experiment logging
- Iteration state tracked in `scratch/NNN-iteration.txt`
- Memory pass between iterations
- Interrupted loop tasks requeued (not completed)
- `cubit memory` command for agent's durable notes

## M11: Structured Logging (planned)

- `results.tsv` structured logging for loop tasks

## M12: Interactive Onboarding (planned)

- Claude-driven `FLUCTLIGHT.md` + `USER.md` generation in `cubit init`
