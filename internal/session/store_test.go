package session

import (
	"testing"
	"time"

	"github.com/frickadelle/agent-relay/internal/config"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	s := New("/tmp/project")
	s.AddTurn("claude", "uuid-1", "what is this repo", "it is a cli tool", "weigh options")
	s.AddTurn("codex", "thread-1", "review it", "looks good", "")
	if err := Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(s.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ID != s.ID || len(got.Turns) != 2 {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if got.LastNativeID("claude") != "uuid-1" {
		t.Errorf("LastNativeID(claude) = %q, want uuid-1", got.LastNativeID("claude"))
	}
	if got.LastNativeID("opencode") != "" {
		t.Errorf("LastNativeID(opencode) = %q, want empty", got.LastNativeID("opencode"))
	}
}

func TestLastNativeIDPrefersLatest(t *testing.T) {
	s := &Session{}
	s.AddTurn("claude", "old", "a", "b", "")
	s.AddTurn("codex", "mid", "c", "d", "")
	s.AddTurn("claude", "new", "e", "f", "")
	if got := s.LastNativeID("claude"); got != "new" {
		t.Errorf("LastNativeID(claude) = %q, want new", got)
	}
}

func TestListSortedNewestFirst(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	old := New("")
	old.CreatedAt = time.Now().Add(-time.Hour)
	newer := New("")
	if err := Save(old); err != nil {
		t.Fatal(err)
	}
	if err := Save(newer); err != nil {
		t.Fatal(err)
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(list))
	}
	if list[0].ID != newer.ID {
		t.Errorf("list[0] = %s, want newest %s", list[0].ID, newer.ID)
	}
}

func TestRemove(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	s := New("")
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	if err := Remove(s.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Load(s.ID); err == nil {
		t.Error("expected load error after remove")
	}
	if err := Remove(s.ID); err == nil {
		t.Error("expected error removing missing session")
	}
}

func TestPreviewTruncation(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	long := make([]byte, 1000)
	for i := range long {
		long[i] = 'x'
	}
	s := New("")
	s.AddTurn("claude", "", string(long), "", "")
	if len(s.Turns[0].PromptPreview) > previewLen+3 {
		t.Errorf("preview not truncated: %d chars", len(s.Turns[0].PromptPreview))
	}
}

func TestThinkingPreviewRoundtrip(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	s := New("/tmp/project")
	s.AddTurn("codex", "thread-1", "review it", "looks good", "checked edge cases")
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Turns[0].ThinkingPreview != "checked edge cases" {
		t.Errorf("ThinkingPreview = %q, want reasoning text", got.Turns[0].ThinkingPreview)
	}
}

func TestRoomRoundtrip(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	s := New("/tmp/project")
	s.Room = "billing-migration"
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Room != "billing-migration" {
		t.Errorf("Room = %q, want billing-migration", got.Room)
	}
}

func TestListFilteredStorageBackends(t *testing.T) {
	for _, backend := range []string{"json", "sqlite"} {
		t.Run(backend, func(t *testing.T) {
			t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
			cfg := config.Default()
			cfg.SessionStorage = backend
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			old := New("/p/api/")
			old.ID = "old"
			old.CreatedAt = time.Now().Add(-time.Hour)
			old.Room = "billing"
			newer := New("/p/api/sub")
			newer.ID = "newer"
			newer.Room = "auth"
			other := New("/p/api-other")
			other.Room = "billing"
			for _, s := range []*Session{old, newer, other} {
				if err := Save(s); err != nil {
					t.Fatal(err)
				}
			}
			for _, tc := range []struct {
				filter Filter
				want   []string
			}{
				{Filter{Workdir: "/p/api"}, []string{"newer", "old"}},
				{Filter{Workdir: "/p/api", Room: "billing"}, []string{"old"}},
				{Filter{Room: "missing"}, nil},
			} {
				list, err := ListFiltered(tc.filter)
				if err != nil {
					t.Fatal(err)
				}
				if len(list) != len(tc.want) {
					t.Fatalf("filter %+v: got %d sessions, want %d", tc.filter, len(list), len(tc.want))
				}
				for i, id := range tc.want {
					if list[i].ID != id {
						t.Fatalf("filter %+v: got %q, want %q", tc.filter, list[i].ID, id)
					}
				}
			}
		})
	}
}
