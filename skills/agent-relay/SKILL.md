---
name: agent-relay
description: Delegate work to other CLI coding agents (claude, codex, opencode), check what agents are currently working on, and run multi-agent roundtable debates. Use when you want a second opinion, a review from another model, token-cheap scouting by a smaller model, or a panel discussion between harnesses. Also use for piping diffs, logs, or files into another agent.
---

# agent-relay

`relay` connects CLI coding harnesses so they can work together. Supported
agents: `claude` (Claude Code), `codex` (Codex CLI), `opencode`.

## Prerequisites

- The `relay` binary is on PATH. If a command fails with "not found", tell the
  user to install it: `go install github.com/frickadelle/agent-relay/cmd/relay@latest`.
- Each harness CLI must be installed and authenticated. Verify with
  `relay doctor` before the first delegation.
- The relay session id goes to stderr, so stdout stays clean for piping.

## Delegate a task

Send a prompt to one agent and print its reply:

```sh
relay ask claude "explain what this repo does"
relay ask opencode --workdir ~/projects/api "find TODOs that reference dead code"
```

Pipe stdin as context, optionally with leading instructions:

```sh
git diff | relay pipe codex "review this diff, list risks first"
cat server.log | relay pipe claude "find the root cause of the crash"
```

Useful flags (also apply to `pipe`):

- `--workdir <dir>`: run the agent in another directory.
- `--model <model>`: model override for this call.
- `--timeout 5m`: per-call timeout (default comes from config).
- `--json`: machine-readable reply with `text`, `thinking`, `session_id`,
  `duration_ms`. Prefer this when you need the session id or the reasoning.
- `--dry-run`: print the exact harness command without running it.
- `--continue <relay-session-id>`: resume the native thread of the addressed
  agent inside an existing relay session.
- `--room <name>`: topic room tag for the session (creates the room on first
  use). `pipe` accepts it too.
- `--no-save`: throwaway call, tracked in no session.

## Sessions

Every `ask`/`pipe` call is tracked in a relay session under
`~/.config/agent-relay/sessions/` (respects `XDG_CONFIG_HOME`).

```sh
relay ask claude "remember the magic word BLORP42"
relay ask claude --continue r_20260821_d787b5 "what was the magic word?"
relay sessions list
relay sessions list --workdir ~/projects/api
relay sessions list --room billing-migration
relay sessions show r_20260821_d787b5
relay sessions rm r_20260821_d787b5
```

A relay session maps each agent to its own native thread (claude session uuid,
codex thread id, opencode session id). `--continue` resumes the native thread
of the agent you address, so follow-ups keep full context without re-sending it.

`sessions list` shows a workdir column per session; `--workdir` filters to one
project directory (including subdirectories), so you find exactly the chats
that worked on a project. `sessions show` prints a thinking preview per turn
when the agent exposed reasoning.

## Rooms

Rooms group sessions by topic across project directories:

```sh
relay ask claude --room billing-migration "draft the rollout plan"
relay rooms list
relay sessions list --room billing-migration
```

Workflow: before starting work on a topic, check `relay rooms list` for an
existing room. If one matches, continue one of its sessions with `--continue
<id>`; otherwise start fresh with `--room <name>`.

A chat can sit in a room and wait for input from another chat:

```sh
echo "migrate the billing tables" | relay rooms post --room billing --from codex
relay rooms wait --room billing
```

`wait` blocks until new messages arrive (`--since N` replays, `--timeout`
gives up, `--json` prints machines). To sit in a room and answer everything,
serve it — one relay session spans all turns, so the harness keeps its native
thread instead of cold-starting per message:

```sh
relay rooms serve --room billing --agent claude --instructions "Reply concisely."
```

`serve` answers each new message and posts replies back as its agent name
(`--as` overrides it). It never answers its own replies. `--max-turns` and
`--idle-timeout` bound the run; Ctrl+C stops.

## Status

`relay status` shows running calls plus recent sessions, so chats sharing a
room or directory do not overlap. Check it before starting file work:

```sh
relay status
relay status --room billing-migration
relay status --workdir ~/projects/api
```

Every `ask`/`pipe` call and roundtable turn announces itself while it runs;
entries expire after 30 minutes, so crashed calls cannot block others.

## Roundtable

Let agents discuss a topic in rounds. Each agent sees what the others said
since its last turn and continues its own native session.

```sh
relay roundtable --agents claude,codex,opencode --rounds 2 \
  "review the migration plan in MIGRATION.md and agree on a rollout order"
```

Rules:

- Each round, every agent speaks once, in the order given.
- At least two agents; max 10 rounds; Ctrl+C stops after the current turn.
- The full transcript is stored as JSONL in
  `~/.config/agent-relay/roundtables/`.
- Experts: give each agent a domain with
  `--roles "claude:security expert,codex:API designer"`. Each agent argues
  from its expert perspective and defers outside its domain.
- Spawning: any harness with shell access can call `relay` itself, so a Codex
  chat can ask a Claude agent via `relay ask claude ...` and join the same
  room, session, or roundtable. The `agent-relay` skill teaches each harness
  this path; the harness needs shell execution allowed.
- Reasoning travels as context: each turn carries the agent's `thinking`
  alongside its answer. Later prompts contain
  `[round N] agent (thinking): ...` lines (capped at 4000 chars per turn,
  tune with `--max-thinking`, `0` disables it; the transcript keeps the full
  thinking), marked as reasoning context only.

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

Codex runs with the `read-only` sandbox unless configured otherwise. Opencode
is invoked with `--thinking` so its reasoning is captured.

## Safety notes

- Agents run with their own default permission settings. In non-interactive
  mode permission-gated actions are denied by the harness itself.
- Use `--dry-run` to show the exact harness command before running it.
- Transcripts and sessions stay local; never paste session files into issues.
