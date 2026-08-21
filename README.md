# agent-relay

`relay` connects CLI coding harnesses so they can work together: send a task to one agent from the terminal, pipe data through them, or let several agents discuss a topic in a round-robin panel.

Supported harnesses:

| Agent | Non-interactive mode | Session resume |
|---|---|---|
| Claude Code | `claude -p ... --output-format json` | `--resume <id>` |
| Codex CLI | `codex exec --json` | `codex exec resume <id>` |
| opencode | `opencode run --format json` | `-s <id>` |

## Install

```sh
go install github.com/frickadelle/agent-relay/cmd/relay@latest
```

Requires the harness CLIs to be installed and authenticated. Check with:

```sh
relay doctor
```

## Delegate a task

```sh
relay ask claude "explain what this repo does"
git diff | relay pipe codex "review this diff, list risks first"
relay ask opencode --workdir ~/projects/api "find TODOs that reference dead code"
```

Useful flags:

- `--workdir <dir>` run the agent in another directory
- `--model <model>` model override
- `--timeout 5m` per-call timeout (default comes from config)
- `--json` machine-readable reply (`text`, `session_id`, `duration_ms`)
- `--dry-run` print the exact command without running it

## Sessions

Every call is tracked in a relay session under `~/.config/agent-relay/sessions/`. The relay session id goes to stderr, so stdout stays clean for piping.

```sh
relay ask claude "remember the magic word BLORP42"
relay ask claude --continue r_20260821_d787b5 "what was the magic word?"
relay sessions list
relay sessions show r_20260821_d787b5
relay sessions rm r_20260821_d787b5
```

A relay session maps each agent to its own native thread (claude session uuid, codex thread id, opencode session id). `--continue` resumes the native thread of the agent you address. Use `--no-save` for throwaway calls.

## Roundtable

Let agents discuss a topic in rounds. Each agent sees what the others said since its last turn and continues its own native session.

```sh
relay roundtable --agents claude,codex,opencode --rounds 2 \
  "review the migration plan in MIGRATION.md and agree on a rollout order"
```

- Each round, every agent speaks once, in the order given.
- Guards: max 10 rounds, per-turn timeout, Ctrl+C stops after the current turn.
- The full transcript is stored as JSONL in `~/.config/agent-relay/roundtables/`.

## Configuration

`~/.config/agent-relay/config.toml` (respects `XDG_CONFIG_HOME`):

```toml
default_timeout = "10m"

[agents.codex]
sandbox = "read-only"   # read-only | workspace-write | danger-full-access

[agents.claude]
timeout = "5m"

[agents.opencode]
model = "opencode/gpt-5-nano"
```

Codex runs with the `read-only` sandbox unless configured otherwise.

## Safety notes

- Agents run with their own default permission settings. In non-interactive mode permission-gated actions are denied by the harness itself; configure each harness separately if you want more autonomy.
- `--dry-run` shows the exact command before you run it.
- Transcripts and sessions stay local.

## Development

```sh
go build ./... && go test ./...
go run ./cmd/relay --help
```

Adapter tests use golden fixtures captured from real harness output (`internal/adapter/testdata/`) plus stub binaries, so no network or API keys are needed.
