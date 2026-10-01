package cli

import (
	"testing"
	"time"

	"github.com/frickadelle/agent-relay/internal/activity"
	"github.com/frickadelle/agent-relay/internal/session"
)

func testSession(id, workdir, room string, at time.Time) *session.Session {
	return &session.Session{ID: id, CreatedAt: at, Workdir: workdir, Room: room}
}

func TestFilterByRoom(t *testing.T) {
	list := []*session.Session{
		testSession("a", "/p", "billing", time.Now()),
		testSession("b", "/p", "", time.Now()),
		testSession("c", "/p", "billing", time.Now()),
	}
	got := filterByRoom(list, "billing")
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("filterByRoom = %v, want sessions a and c", got)
	}
	if got := filterByRoom(list, "missing"); len(got) != 0 {
		t.Errorf("filterByRoom(missing) = %v, want empty", got)
	}
}

func TestFilterByWorkdir(t *testing.T) {
	list := []*session.Session{
		testSession("a", "/projects/api", "", time.Now()),
		testSession("b", "/projects/api/sub", "", time.Now()),
		testSession("c", "/projects/other", "", time.Now()),
		testSession("d", "", "", time.Now()),
	}
	got := filterByWorkdir(list, "/projects/api")
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("filterByWorkdir = %v, want sessions a and b", got)
	}
}

func TestParseRoles(t *testing.T) {
	roles, err := parseRoles("claude:security expert,codex:API designer")
	if err != nil {
		t.Fatal(err)
	}
	if roles["claude"] != "security expert" || roles["codex"] != "API designer" {
		t.Errorf("parseRoles = %v", roles)
	}
	if roles, err := parseRoles(""); err != nil || roles != nil {
		t.Errorf("parseRoles(\"\") = %v, %v; want nil, nil", roles, err)
	}
	for _, bad := range []string{"claude", "claude:", ":expert"} {
		if _, err := parseRoles(bad); err == nil {
			t.Errorf("parseRoles(%q) should fail", bad)
		}
	}
}

func TestFilterActive(t *testing.T) {
	now := time.Now()
	list := []activity.Active{
		{Agent: "codex", Room: "billing", Workdir: "/projects/api", StartedAt: now},
		{Agent: "claude", Room: "auth", Workdir: "/projects/other", StartedAt: now},
		{Agent: "opencode", Workdir: "/projects/api/sub", StartedAt: now},
	}
	if got := filterActive(list, "billing", ""); len(got) != 1 || got[0].Agent != "codex" {
		t.Errorf("filterActive(room) = %+v, want codex only", got)
	}
	if got := filterActive(list, "", "/projects/api"); len(got) != 2 {
		t.Errorf("filterActive(workdir) = %+v, want codex and opencode", got)
	}
	if got := filterActive(list, "billing", "/projects/other"); len(got) != 0 {
		t.Errorf("filterActive(both) = %+v, want empty", got)
	}
}

func TestSummarizeRooms(t *testing.T) {
	now := time.Now()
	list := []*session.Session{
		testSession("a", "/p", "billing", now.Add(-time.Hour)),
		testSession("b", "/p", "", now),
		testSession("c", "/p", "billing", now),
		testSession("d", "/p", "auth", now.Add(-2*time.Hour)),
	}
	rooms := summarizeRooms(list)
	if len(rooms) != 2 {
		t.Fatalf("len(rooms) = %d, want 2", len(rooms))
	}
	if rooms[0].Name != "billing" || rooms[0].Sessions != 2 {
		t.Errorf("rooms[0] = %+v, want billing with 2 sessions", rooms[0])
	}
	if rooms[1].Name != "auth" || rooms[1].Sessions != 1 {
		t.Errorf("rooms[1] = %+v, want auth with 1 session", rooms[1])
	}
}
