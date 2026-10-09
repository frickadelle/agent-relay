package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/frickadelle/agent-relay/internal/config"
)

func enableSQLite(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AGENT_RELAY_CONFIG_DIR", dir)
	cfg := config.Default()
	cfg.SessionStorage = "sqlite"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSQLiteRoundtripFiltersAndRemove(t *testing.T) {
	enableSQLite(t)
	now := time.Now()
	for i, wd := range []string{"/p/api/", "/p/api/sub", "/p/api-other", "/p/100%_real", "/p/100XXreal"} {
		s := New(wd)
		s.ID = string(rune('a' + i))
		s.CreatedAt = now.Add(time.Duration(i) * time.Minute)
		s.Room = "billing"
		if i == 1 {
			s.Room = "auth"
		}
		s.AddTurn("codex", "thread-1", "prompt", "reply", "thinking")
		s.AddTurn("claude", "uuid-1", "follow up", "done", "")
		if err := Save(s); err != nil {
			t.Fatal(err)
		}
		got, err := Load(s.ID)
		if err != nil {
			t.Fatal(err)
		}
		expected, _ := json.Marshal(s)
		actual, _ := json.Marshal(got)
		if string(actual) != string(expected) {
			t.Fatalf("roundtrip: %+v", got)
		}
	}
	list, err := ListFiltered(Filter{Workdir: "/p/api"})
	if err != nil || len(list) != 2 || list[0].ID != "b" || list[1].ID != "a" {
		t.Fatalf("directory filter: %+v, %v", list, err)
	}
	list, err = ListFiltered(Filter{Workdir: "/p/api", Room: "billing"})
	if err != nil || len(list) != 1 || list[0].ID != "a" {
		t.Fatalf("combined filter: %+v, %v", list, err)
	}
	list, err = ListFiltered(Filter{Workdir: "/p/100%_real"})
	if err != nil || len(list) != 1 || list[0].ID != "d" {
		t.Fatalf("literal filter: %+v, %v", list, err)
	}
	if _, err := Load(""); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty ID: %v", err)
	}
	if err := Remove("a"); err != nil {
		t.Fatal(err)
	}
	if err := Remove("a"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove missing: %v", err)
	}
	db, err := openSQLite()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM turns WHERE session_id='a'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphan turns: %d, %v", count, err)
	}
}

func TestSQLiteImportsOnceAndPreservesJSON(t *testing.T) {
	dir := enableSQLite(t)
	s := New("/p")
	s.Room = "legacy"
	s.Turns = []Turn{{Agent: "claude", NativeID: "uuid", PromptPreview: "old", At: time.Now()}}
	if err := jsonSave(s); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path(s.ID))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastNativeID("claude") != "uuid" || len(got.Turns) != 1 {
		t.Fatalf("import: %+v", got)
	}
	got.AddTurn("codex", "thread", "new", "answer", "")
	if err := Save(got); err != nil {
		t.Fatal(err)
	}
	if err := Save(got); err != nil {
		t.Fatal(err)
	}
	got, err = Load(s.ID)
	if err != nil || len(got.Turns) != 2 {
		t.Fatalf("retry duplicates: %+v, %v", got, err)
	}
	after, err := os.ReadFile(path(s.ID))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("legacy file changed")
	}
	if _, err := os.Stat(filepath.Join(dir, "sessions.db")); err != nil {
		t.Fatal(err)
	}
	if err := Remove(s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(s.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted session reimported: %v", err)
	}
	cfg := config.Default()
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	legacy, err := Load(s.ID)
	if err != nil || len(legacy.Turns) != 1 {
		t.Fatalf("JSON rollback: %+v, %v", legacy, err)
	}
}

func TestSQLiteImportFailureRollsBackAndRetries(t *testing.T) {
	enableSQLite(t)
	s := New("/p")
	s.ID = "a_valid"
	if err := jsonSave(s); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(Dir(), "z_broken.json")
	if err := os.WriteFile(bad, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := List(); err == nil {
		t.Fatal("expected corrupt import failure")
	}
	if err := os.Remove(bad); err != nil {
		t.Fatal(err)
	}
	list, err := List()
	if err != nil || len(list) != 1 || list[0].ID != s.ID {
		t.Fatalf("retry: %+v, %v", list, err)
	}
}

func TestSQLiteConcurrentAppends(t *testing.T) {
	enableSQLite(t)
	s := New("/p")
	s.AddTurn("claude", "base", "first", "reply", "")
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	const writers = 8
	copies := make([]*Session, writers)
	for i := range copies {
		var err error
		copies[i], err = Load(s.ID)
		if err != nil {
			t.Fatal(err)
		}
		copies[i].AddTurn("codex", string(rune('a'+i)), "next", "answer", "")
	}
	var wg sync.WaitGroup
	failures := make(chan error, writers)
	for _, copy := range copies {
		wg.Go(func() { failures <- Save(copy) })
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := Load(s.ID)
	if err != nil || len(got.Turns) != writers+1 {
		t.Fatalf("lost turns: %+v, %v", got, err)
	}
	for _, copy := range copies {
		if err := Save(copy); err != nil {
			t.Fatal(err)
		}
	}
	got, err = Load(s.ID)
	if err != nil || len(got.Turns) != writers+1 {
		t.Fatalf("retry duplicated turns: %+v, %v", got, err)
	}
}

func TestSQLiteConcurrentInitialization(t *testing.T) {
	enableSQLite(t)
	s := New("/p")
	s.AddTurn("claude", "old", "prompt", "reply", "")
	if err := jsonSave(s); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			got, err := Load(s.ID)
			if err != nil {
				t.Error(err)
				return
			}
			if len(got.Turns) != 1 {
				t.Errorf("imported %d turns", len(got.Turns))
			}
		})
	}
	wg.Wait()
}

func TestSQLiteSaveFailureIsAtomic(t *testing.T) {
	enableSQLite(t)
	s := New("/p")
	s.Room = "before"
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	db, err := openSQLite()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TRIGGER reject_turn BEFORE INSERT ON turns WHEN NEW.native_id = 'reject' BEGIN SELECT RAISE(ABORT, 'rejected turn'); END`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	s.Room = "after"
	s.AddTurn("codex", "ok", "first", "reply", "")
	s.AddTurn("codex", "reject", "second", "reply", "")
	if err := Save(s); err == nil {
		t.Fatal("expected failed transaction")
	}
	got, err := Load(s.ID)
	if err != nil || got.Room != "before" || len(got.Turns) != 0 {
		t.Fatalf("partial save: %+v, %v", got, err)
	}
	db, err = openSQLite()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("DROP TRIGGER reject_turn")
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	got, err = Load(s.ID)
	if err != nil || got.Room != "after" || len(got.Turns) != 2 {
		t.Fatalf("retry: %+v, %v", got, err)
	}
}

func BenchmarkSessionRoomFilter(b *testing.B) {
	for _, backend := range []string{"json", "sqlite"} {
		b.Run(backend, func(b *testing.B) {
			b.Setenv("AGENT_RELAY_CONFIG_DIR", b.TempDir())
			cfg := config.Default()
			cfg.SessionStorage = backend
			if err := cfg.Save(); err != nil {
				b.Fatal(err)
			}
			for i := range 1000 {
				s := New("/p")
				s.ID = fmt.Sprintf("session_%04d", i)
				s.Room = fmt.Sprintf("room_%d", i%100)
				s.AddTurn("codex", "thread", "prompt", "reply", "")
				if err := Save(s); err != nil {
					b.Fatal(err)
				}
			}
			b.ResetTimer()
			for b.Loop() {
				list, err := ListFiltered(Filter{Room: "room_42"})
				if err != nil || len(list) != 10 {
					b.Fatalf("filter: %d sessions, %v", len(list), err)
				}
			}
		})
	}
}
