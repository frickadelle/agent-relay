# agent-relay

```text
              Codex                            Claude                            OpenCode

              ..                               .::.
             +:+:                              :+.#
             .++.                     ::        :#.       ::            ..                      :
          ....++.....                  +:   .:::::::::.  :+             +#.     ::::::::+:     +#.
     :+##+:::::+:::+++#+::          :.    :+....::....:+    .:          ::+   :+:     :+:.+:  +:+
    .+  ..:+..............+         .::  +: :#@@#+@@#+ .+. ::.          ::::::++    .+:    .++:::
    :#::  ::.+#####@@@@@# +            .+: #@+.#: @::@@. #.             .+:+   ::::::      .+@.+
   :::+:+ :::@++@@@@@@@@#.+           :+# +@##:.. ..+#@@ +++             ## :+++++:++++:. :++:++
   #  + # :.:@#: +@@@@@@#.+           +.+ #@:.:.   ::.#@ :.+            ::+.@@##@@@@##@@# +.+  +
   +:.+:+ +.:@+:+#+++#@@#.+           .+#..@@#..: :.+@@+ #+:            :++.@@  +@@#  @@# +:+.::
    :@::  +.+@@@@@+##@@@+.+             .+.:##+@+ @#+@+ +:    .  ..      :#.+@++@@@@:+@@@  :+#:
     +:.. +..++:::::::::.+:               +..:+##+##+..+.    :+++:+.      .+::++++++##+:  .::
      .:+##+++++++:++:....              ..:++::::::::+#:.   .# .. +.        :::.........:#+:
  .:+#+:::+..::::. ++#.       ...      .@+ ++ :++++: +:.@+ :::::::....       :##::####+:++.:+#++:.
 .@@@@:+#:.+       .@##::::::::::#.   .+:+++:::....:::::#.#.  :#  +::::::::::##:        # #++#@@@@
 .@@@#+:.+#.   ..  ::+.          +   .#+.:@:            + +..:+   +.         .@+   .    +#:.:@@@@@
  @@@@+ .++::+:::+:+++    :@:   +:   #: :+++    .@+    .#::#+:    .+   :@:    ##.+:.:++::#:  @@@@+
  +@#@#.#   .# :::###.     .    #::::+::#+##     ..    :+::::::::::#.   .     .+##::.:#   :+.@#@@.
  .+ :##+::::++##+###.......:::+:      ...:#:::::..::::+.          .+:::.......@#####:+::::###..#
  .+.#.           ..::........              ...........               ........::..          .:+.+
   +:+.:::.....                                                                      .....::..+:+
   ++#+:.......:::::::::::................................................:::::::::.........:+##:
   .@@@@@@#++++:::::.................................::.........................:::+++++##@@@@@#
    +#.::#@. .@++@:..:#....:::##+:::::::::::..:+:::+:..:::::::::::+##::::...:+...#####  +@+:.:@.
    @:   +#  +.  +:..:#      .##:         +#+#+#. .#+#+#+         .@#:      ++:..#   +. .@    #+
   .@    #+ .#.::+#++::+:    +.+          +:...+: :+...:+          +.+    .:+:+++@::.:+  @:   :@
   ++    #. +:.   :+    +:   +:+         :+    .# +:    ++         ::+.   #.   :+   ..#. +:    #.
         + .#:....:+:::::.  ::+.          ::::::. .::::::.          +::   :::::++.....+# ::
        .#.#.......         :+:                                     :++          ..... +:.+
         .:.                 .                                       .                  ::.
```

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
relay sessions list --workdir ~/projects/api
relay sessions list --room billing-migration
relay sessions show r_20260821_d787b5
relay sessions rm r_20260821_d787b5
```

A relay session maps each agent to its own native thread (claude session uuid, codex thread id, opencode session id). `--continue` resumes the native thread of the agent you address. Use `--no-save` for throwaway calls.

`sessions list` shows the workdir column so you can see which chat belongs to which project; `--workdir` filters the list to one project directory (including subdirectories). `sessions show` prints a thinking preview per turn when the agent exposed reasoning. Older session files without these fields keep loading unchanged.

## Rooms

Rooms group sessions by topic across project directories. Tag a session with `--room` (the room is created on first use); `pipe` accepts the flag too since it shares the ask flags.

```sh
relay ask claude --room billing-migration "draft the rollout plan"
relay rooms list
relay sessions list --room billing-migration
```

Workflow: before starting work on a topic, check `relay rooms list`. If a room exists, continue one of its sessions with `--continue <id>`; otherwise start fresh with `--room <name>`.

A chat can also sit in a room and wait for input from another chat. Post full messages to the room feed and wait for new ones:

```sh
echo "migrate the billing tables" | relay rooms post --room billing --from codex
relay rooms wait --room billing
```

`wait` blocks until messages arrive (default: only new ones; `--since N` replays after sequence N, `--timeout` gives up, `--json` prints machines). To sit in a room and answer everything, serve it — one relay session spans all turns, so the harness keeps its native thread instead of cold-starting per message:

```sh
relay rooms serve --room billing --agent claude --instructions "Reply concisely."
```

`serve` answers each new message with the same native thread and posts replies back as its agent name (`--as` overrides it). It never answers its own replies. `--max-turns` and `--idle-timeout` bound the run; Ctrl+C stops. Room names allow letters, digits, `-`, `_`.

## Status

`relay status` shows which chats work on what right now (running calls) plus recent sessions, so chats do not overlap:

```sh
relay status
relay status --room billing-migration
relay status --workdir ~/projects/api
```

Every `ask`/`pipe` call and roundtable turn announces itself while it runs and removes the entry when done. Entries older than 30 minutes expire automatically, so crashed calls cannot block others. Check status before starting file work in a shared room or directory.

## Roundtable

Let agents discuss a topic in rounds. Each agent sees what the others said since its last turn and continues its own native session.

```sh
relay roundtable --agents claude,codex,opencode --rounds 2 \
  "review the migration plan in MIGRATION.md and agree on a rollout order"
```

- Each round, every agent speaks once, in the order given.
- Guards: max 10 rounds, per-turn timeout, Ctrl+C stops after the current turn.
- The full transcript is stored as JSONL in `~/.config/agent-relay/roundtables/`.
- Experts: assign roles with `--roles "claude:security expert,codex:API designer"`.
  Each agent argues from its expert perspective and defers outside its domain.
  Unknown agent names are rejected.
- Reasoning context: each turn stores the agent's thinking (`thinking`) alongside
  its answer (`text`). The next agents receive prior thinking as
  `[round N] agent (thinking): ...` context lines, capped at 4000 chars per
  turn (tune with `--max-thinking`, `0` disables thinking context; the
  transcript always keeps the full thinking).
  Codex reasoning items and opencode reasoning parts are captured automatically
  (`opencode run` is invoked with `--thinking`); Claude thinking is captured
  when the harness emits assistant thinking blocks. `relay ask --json` also
  returns the `thinking` field.

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

## Agent skill

`skills/agent-relay/SKILL.md` teaches a harness how to use relay (delegate,
pipe, roundtable). Install it by linking the folder into your skills directory:

```sh
ln -s ~/Developer/agent-relay/skills/agent-relay ~/.claude/skills/agent-relay
ln -s ~/Developer/agent-relay/skills/agent-relay ~/.codex/skills/agent-relay
```

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
