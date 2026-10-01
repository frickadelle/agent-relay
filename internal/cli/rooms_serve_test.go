package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/frickadelle/agent-relay/internal/adapter"
	"github.com/frickadelle/agent-relay/internal/feed"
	"github.com/frickadelle/agent-relay/internal/session"
)

type serveStub struct {
	name    string
	replies []string
	calls   int
	native  []string
	prompts []string
}

func (s *serveStub) Name() string  { return s.name }
func (s *serveStub) Detect() error { return nil }
func (s *serveStub) Command(req adapter.Request) ([]string, error) {
	return []string{s.name}, nil
}
func (s *serveStub) Send(ctx context.Context, req adapter.Request) (adapter.Reply, error) {
	s.calls++
	s.native = append(s.native, req.Session)
	s.prompts = append(s.prompts, req.Prompt)
	return adapter.Reply{
		Agent:     s.name,
		Text:      s.replies[(s.calls-1)%len(s.replies)],
		SessionID: s.name + "-native",
	}, nil
}

func TestServeAnswersAndPostsBack(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	stub := &serveStub{name: "claude", replies: []string{"on it"}}
	type result struct {
		sess *session.Session
		err  error
	}
	done := make(chan result, 1)
	go func() {
		sess, err := serveLoop(context.Background(), serveConfig{
			agent: stub, room: "billing", instructions: "Reply concisely.",
			timeout: time.Minute, maxTurns: 1, poll: 10 * time.Millisecond,
		})
		done <- result{sess, err}
	}()
	time.Sleep(50 * time.Millisecond)
	if _, err := feed.Post("billing", "codex", "migrate the tables"); err != nil {
		t.Fatal(err)
	}
	var res result
	select {
	case res = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not answer in time")
	}
	if res.err != nil {
		t.Fatal(res.err)
	}
	if stub.calls != 1 {
		t.Fatalf("calls = %d, want 1", stub.calls)
	}
	if !strings.Contains(stub.prompts[0], "Reply concisely.") ||
		!strings.Contains(stub.prompts[0], "migrate the tables") {
		t.Errorf("prompt = %q, want instructions plus inbox", stub.prompts[0])
	}
	sess := res.sess
	if len(sess.Turns) != 1 || sess.Room != "billing" {
		t.Errorf("session = %+v, want one turn tagged billing", sess)
	}
	entries, cursor, err := feed.Read("billing", 1)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 2 || len(entries) != 1 || entries[0].From != "claude" || entries[0].Text != "on it" {
		t.Errorf("feed = %+v, cursor %d; want claude reply posted", entries, cursor)
	}
	if _, err := session.Load(sess.ID); err != nil {
		t.Errorf("session should be saved: %v", err)
	}
}

func TestServeResumesNativeThread(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	stub := &serveStub{name: "claude", replies: []string{"a1", "a2"}}
	finished := make(chan error, 1)
	go func() {
		_, err := serveLoop(context.Background(), serveConfig{
			agent: stub, room: "billing", timeout: time.Minute,
			maxTurns: 2, poll: 10 * time.Millisecond,
		})
		finished <- err
	}()
	time.Sleep(50 * time.Millisecond)
	if _, err := feed.Post("billing", "codex", "one"); err != nil {
		t.Fatal(err)
	}
	waitFeedLen(t, "billing", 2)
	if _, err := feed.Post("billing", "codex", "two"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not finish in time")
	}
	if len(stub.native) != 2 {
		t.Fatalf("native sessions = %v", stub.native)
	}
	if stub.native[0] != "" {
		t.Errorf("first turn should start a fresh native thread, got %q", stub.native[0])
	}
	if stub.native[1] != "claude-native" {
		t.Errorf("second turn should resume claude-native, got %q", stub.native[1])
	}
	if strings.Contains(stub.prompts[1], "a1") {
		t.Errorf("serve answered its own reply: %q", stub.prompts[1])
	}
}

// waitFeedLen polls until the feed holds n lines or the test times out.
func waitFeedLen(t *testing.T, room string, n int) {
	t.Helper()
	for i := 0; i < 200; i++ {
		_, cursor, err := feed.Read(room, 0)
		if err != nil {
			t.Fatal(err)
		}
		if cursor >= n {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("feed %q did not reach %d lines in time", room, n)
}

func TestServeIdleTimeoutStops(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	stub := &serveStub{name: "claude", replies: []string{"x"}}
	sess, err := serveLoop(context.Background(), serveConfig{
		agent: stub, room: "quiet", idleTimeout: 50 * time.Millisecond,
		poll: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 0 {
		t.Errorf("calls = %d, want 0 on idle room", stub.calls)
	}
	if len(sess.Turns) != 0 {
		t.Errorf("turns = %d, want 0", len(sess.Turns))
	}
}
