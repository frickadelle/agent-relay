# AGENTS.md

`relay` is a Go CLI (cobra) that connects coding harnesses (Claude Code, Codex
CLI, opencode) for task delegation and roundtable debates.

## Commands

```sh
go build ./...
go vet ./...
go test ./...
go run ./cmd/relay --help
```

If the build fails on VCS stamping in this environment, retry with
`-buildvcs=false` (e.g. `go build -buildvcs=false ./...`).

## Layout

- `cmd/relay/` entry point, `internal/cli/` commands
- `internal/adapter/` harness adapters, golden fixtures in `testdata/`
- `internal/session/`, `internal/roundtable/`, `internal/config/`
- `skills/agent-relay/SKILL.md` teaches harnesses how to use relay

## Conventions

- English for all code, comments, docs, and commit messages.
- New JSON fields on persisted structs (sessions, transcripts) must be
  `omitempty` so old files keep loading.
- Adapter tests use golden fixtures plus stub binaries; no network or API keys.

## Multi-agent workflow

- Check `relay rooms list` before starting topic work.
- Check `relay status --room <name>` before file work to avoid overlapping
  with another active chat.
- Reuse a room via `relay ask <agent> --continue <id>`; start new work with
  `--room <name>`. To wait for another chat, use `relay rooms wait/post`;
  to answer a room continuously, use `relay rooms serve`
  (see `skills/agent-relay/SKILL.md`).
- See `skills/agent-relay/SKILL.md` for delegation recipes.
