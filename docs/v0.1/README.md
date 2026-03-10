# v0.1 — Project Skeleton + Agent Init

**Milestones:** M0, M1
**Status:** Shipped

## M0: Project Skeleton + Config

- Go CLI with Cobra + Viper
- `cmd/root.go` with config loading via `PersistentPreRunE`
- `internal/config/` with `Load()`, `Default()`, `AgentDir()`
- Config file: `~/.ark/cubit/config.yaml`
- `cubit version` and `cubit config show` commands
- GoReleaser + GitHub Actions release pipeline

## M1: `cubit init` with Onboarding

- `internal/scaffold/` — `Init()` creates agent directory structure
- `internal/scaffold/setup.go` — `RunSetup()` for LLM-driven onboarding
- Generates `FLUCTLIGHT.md`, `USER.md`, `GOALS.md` in `identity/`
- Flags: `--skip-onboard`, `--import-identity`, `--force`
