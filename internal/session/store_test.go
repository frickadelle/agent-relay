package session

import (
	"testing"
	"time"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	s := New("/tmp/project")
	s.AddTurn("claude", "uuid-1", "what is this repo", "it is a cli tool")
	s.AddTurn("codex", "thread-1", "review it", "looks good")
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
	s.AddTurn("claude", "old", "a", "b")
	s.AddTurn("codex", "mid", "c", "d")
	s.AddTurn("claude", "new", "e", "f")
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
	s.AddTurn("claude", "", string(long), "")
	if len(s.Turns[0].PromptPreview) > previewLen+3 {
		t.Errorf("preview not truncated: %d chars", len(s.Turns[0].PromptPreview))
	}
}
