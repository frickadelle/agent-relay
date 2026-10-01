# CLAUDE.md

Project instructions live in AGENTS.md — follow that file.

Quick reference: `go build ./... && go vet ./... && go test ./...` (add
`-buildvcs=false` if VCS stamping fails in this environment). Keep new JSON
fields `omitempty` so old session and transcript files keep loading.
Coordinate multi-agent work via `relay rooms list` and `--room` as described
in AGENTS.md and `skills/agent-relay/SKILL.md`.
