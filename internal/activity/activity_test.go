package activity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrackListCleanup(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	done := Track(Active{Agent: "codex", Room: "billing", Prompt: "review it"})
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Agent != "codex" || list[0].Room != "billing" {
		t.Fatalf("List = %+v, want one codex entry", list)
	}
	done()
	if list, err := List(); err != nil || len(list) != 0 {
		t.Errorf("List after cleanup = %+v, %v; want empty", list, err)
	}
}

func TestListPrunesStale(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_RELAY_CONFIG_DIR", dir)
	done := Track(Active{Agent: "claude", Prompt: "old work"})
	_ = done
	// Backdate the file beyond TTL.
	entries, _ := os.ReadDir(Dir())
	if len(entries) != 1 {
		t.Fatalf("want one entry, got %d", len(entries))
	}
	stale := time.Now().Add(-TTL - time.Minute)
	path := filepath.Join(Dir(), entries[0].Name())
	// Rewrite content with an old StartedAt (content decides, not mtime).
	old := Active{ID: "stale", Agent: "claude", StartedAt: stale}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("List = %+v, want stale entry pruned", list)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("stale file should be removed from disk")
	}
}
