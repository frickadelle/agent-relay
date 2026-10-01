package feed

import (
	"context"
	"testing"
	"time"
)

func TestPostReadSeq(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	first, err := Post("billing", "codex", "hello")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Post("billing", "claude", "hi back")
	if err != nil {
		t.Fatal(err)
	}
	if first.Seq != 1 || second.Seq != 2 {
		t.Errorf("seqs = %d, %d; want 1, 2", first.Seq, second.Seq)
	}
	entries, cursor, err := Read("billing", 0)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 2 || len(entries) != 2 || entries[1].From != "claude" {
		t.Errorf("Read = %+v, cursor %d", entries, cursor)
	}
	entries, cursor, err = Read("billing", 1)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 2 || len(entries) != 1 || entries[0].Seq != 2 {
		t.Errorf("Read(since 1) = %+v, cursor %d", entries, cursor)
	}
}

func TestPostRejects(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	for _, room := range []string{"", "../evil", "a/b", "x!y"} {
		if _, err := Post(room, "codex", "hi"); err == nil {
			t.Errorf("Post(%q) should fail", room)
		}
	}
	if _, err := Post("ok", "", "hi"); err == nil {
		t.Error("Post without from should fail")
	}
	if _, err := Post("ok", "codex", "  "); err == nil {
		t.Error("Post without text should fail")
	}
}

func TestWaitImmediate(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	if _, err := Post("billing", "codex", "hello"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entries, cursor, err := Wait(ctx, "billing", 0, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 1 || len(entries) != 1 {
		t.Errorf("Wait = %+v, cursor %d", entries, cursor)
	}
}

func TestWaitDelayedPost(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = Post("billing", "codex", "late hello")
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entries, _, err := Wait(ctx, "billing", 0, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Text != "late hello" {
		t.Errorf("Wait = %+v", entries)
	}
}

func TestWaitTimeout(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, _, err := Wait(ctx, "empty", 0, 10*time.Millisecond); err == nil {
		t.Error("Wait should time out on empty feed")
	}
}
