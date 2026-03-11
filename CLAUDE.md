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

- `cmd/` — Cobra root + commands (init, migrate, status, edit, archive, version, update)
- `internal/config/` — Config types + loading via Viper
- `internal/scaffold/` — Agent workspace scaffolding
- `internal/updater/` — GitHub release self-updater
- `main.go` — Entry point

## Conventions

- Module: `github.com/SeanoChang/cubit`
- Config file: `~/.ark/config.yaml`
- Agent data: `~/.ark/<agent>/` (e.g. `~/.ark/noah/`)
- Each agent dir is a git repo with `.claude/` config
- Version injected via ldflags at build time
- Release targets: linux/amd64 + darwin/arm64
