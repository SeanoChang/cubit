# Cubit

Agent control plane. Go CLI built with Cobra + Viper.

## Build & Run

```bash
go build -o cubit .
./cubit version
./cubit config show
```

## Project Layout

- `cmd/` — Cobra commands (one file per command/subcommand)
- `internal/config/` — Config types + loading via Viper
- `main.go` — Entry point

## Conventions

- Module: `github.com/SeanoChang/cubit`
- Config file: `~/.ark/cubit/config.yaml`
- Agent data: `~/.ark/cubit/<agent>/` (e.g. `~/.ark/cubit/noah/`)
- Version injected via ldflags at build time
- Release targets: linux/amd64 + darwin/arm64
