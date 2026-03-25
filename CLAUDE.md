# Cubit

Agent workspace filesystem CLI. Go CLI built with Cobra + Viper.

## Build & Run

```bash
go build -o cubit .
./cubit version
./cubit init noah
./cubit status
```

## Project Layout

- `cmd/` — Cobra root + commands (init, migrate, migrate-projects, status, edit, archive, project, version, update)
- `internal/config/` — Config types + loading via Viper
- `internal/scaffold/` — Agent workspace scaffolding
- `internal/project/` — Project CRUD, search, git operations
- `internal/updater/` — GitHub release self-updater
- `main.go` — Entry point

## Conventions

- Module: `github.com/SeanoChang/cubit`
- Config file: `~/.ark/config.yaml`
- Agent data: `~/.ark/agents-home/<agent>/` (e.g. `~/.ark/agents-home/noah/`)
- Agent dirs are plain directories (not git repos); git repos live in `projects/` subdirectories
- Each agent dir has `.claude/` config
- Version injected via ldflags at build time
- Release targets: linux/amd64 + darwin/arm64
