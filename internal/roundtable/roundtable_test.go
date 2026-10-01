package roundtable

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/frickadelle/agent-relay/internal/adapter"
)

type fakeAdapter struct {
	name     string
	replies  []string
	thinking []string
	calls    int
	sessions []string
	failAt   int
}

func (f *fakeAdapter) Name() string { return f.name }

func (f *fakeAdapter) Detect() error { return nil }

func (f *fakeAdapter) Command(req adapter.Request) ([]string, error) { return nil, nil }

func (f *fakeAdapter) Send(ctx context.Context, req adapter.Request) (adapter.Reply, error) {
	f.calls++
	f.sessions = append(f.sessions, req.Session)
	if f.failAt > 0 && f.calls >= f.failAt {
		return adapter.Reply{}, fmt.Errorf("boom on call %d", f.calls)
	}
	text := f.replies[(f.calls-1)%len(f.replies)]
	think := ""
	if len(f.thinking) > 0 {
		think = f.thinking[(f.calls-1)%len(f.thinking)]
	}
	return adapter.Reply{
		Agent:     f.name,
		Text:      text,
		Thinking:  think,
		SessionID: fmt.Sprintf("%s-native-%d", f.name, f.calls),
	}, nil
}

func twoFakes() (*fakeAdapter, *fakeAdapter) {
	return &fakeAdapter{name: "alpha", replies: []string{"alpha speaks"}},
		&fakeAdapter{name: "beta", replies: []string{"beta speaks"}}
}

func TestRunTurnOrderAndResume(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	a, b := twoFakes()
	p := &Panel{Agents: []adapter.Adapter{a, b}, Topic: "testing", Rounds: 2}

	var turns []Turn
	p.OnTurn = func(tu Turn) { turns = append(turns, tu) }

	result, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result) != 4 {
		t.Fatalf("got %d turns, want 4", len(result))
	}
	wantOrder := []string{"alpha", "beta", "alpha", "beta"}
	for i, w := range wantOrder {
		if result[i].Agent != w {
			t.Errorf("turn %d agent = %q, want %q", i, result[i].Agent, w)
		}
		if result[i].Round != i/2+1 {
			t.Errorf("turn %d round = %d", i, result[i].Round)
		}
	}

	if a.sessions[0] != "" {
		t.Errorf("first alpha call should have no native session, got %q", a.sessions[0])
	}
	if a.sessions[1] != "alpha-native-1" {
		t.Errorf("second alpha call should resume alpha-native-1, got %q", a.sessions[1])
	}
	if b.sessions[1] != "beta-native-1" {
		t.Errorf("second beta call should resume beta-native-1, got %q", b.sessions[1])
	}

	if !strings.Contains(p.TranscriptPath, "roundtables") || !strings.HasSuffix(p.TranscriptPath, ".jsonl") {
		t.Errorf("TranscriptPath = %q", p.TranscriptPath)
	}
}

func TestRunValidatesRounds(t *testing.T) {
	a, _ := twoFakes()
	p := &Panel{Agents: []adapter.Adapter{a}, Topic: "x", Rounds: 0}
	if _, err := p.Run(context.Background()); err == nil {
		t.Error("expected error for rounds=0")
	}
	p.Rounds = MaxRounds + 1
	if _, err := p.Run(context.Background()); err == nil {
		t.Error("expected error for rounds above cap")
	}
	p = &Panel{Topic: "x", Rounds: 2}
	if _, err := p.Run(context.Background()); err == nil {
		t.Error("expected error for no agents")
	}
}

func TestRunStopsOnAdapterError(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	a, _ := twoFakes()
	bad := &fakeAdapter{name: "bad", replies: []string{"x"}, failAt: 1}
	p := &Panel{Agents: []adapter.Adapter{a, bad}, Topic: "x", Rounds: 3}
	result, err := p.Run(context.Background())
	if err == nil {
		t.Fatal("expected error from failing adapter")
	}
	if len(result) != 1 {
		t.Errorf("got %d turns before failure, want 1", len(result))
	}
}

func TestTurnPromptContainsDelta(t *testing.T) {
	p := &Panel{Topic: "gophers"}
	first := p.turnPrompt("claude", nil, true)
	if !strings.Contains(first, `You are "claude"`) || !strings.Contains(first, "gophers") {
		t.Errorf("first prompt malformed: %q", first)
	}
	delta := []Turn{{Round: 1, Agent: "codex", Text: "I like rust"}}
	next := p.turnPrompt("claude", delta, false)
	if !strings.Contains(next, "[round 1] codex: I like rust") {
		t.Errorf("delta prompt missing transcript line: %q", next)
	}
}

func TestTurnPromptContainsThinking(t *testing.T) {
	p := &Panel{Topic: "gophers", MaxThinking: DefaultMaxThinking}
	delta := []Turn{{Round: 1, Agent: "codex", Text: "done", Thinking: "checked edge cases"}}
	next := p.turnPrompt("claude", delta, false)
	if !strings.Contains(next, "[round 1] codex: done") {
		t.Errorf("prompt missing answer line: %q", next)
	}
	if !strings.Contains(next, "[round 1] codex (thinking): checked edge cases") {
		t.Errorf("prompt missing thinking context: %q", next)
	}
}

func TestTurnPromptThinkingDisabled(t *testing.T) {
	p := &Panel{Topic: "gophers", MaxThinking: 0}
	delta := []Turn{{Round: 1, Agent: "codex", Text: "done", Thinking: "checked edge cases"}}
	next := p.turnPrompt("claude", delta, false)
	if strings.Contains(next, "(thinking)") {
		t.Errorf("prompt should omit thinking when disabled: %q", next)
	}
	if !strings.Contains(next, "[round 1] codex: done") {
		t.Errorf("prompt missing answer line: %q", next)
	}
}

func TestTurnPromptThinkingTruncated(t *testing.T) {
	p := &Panel{Topic: "gophers", MaxThinking: 10}
	delta := []Turn{{Round: 1, Agent: "codex", Text: "done", Thinking: "0123456789abcdef"}}
	next := p.turnPrompt("claude", delta, false)
	if strings.Contains(next, "abcdef") {
		t.Errorf("prompt should truncate thinking: %q", next)
	}
	if !strings.Contains(next, "(thinking truncated)") {
		t.Errorf("prompt missing truncation marker: %q", next)
	}
}

func TestTurnPromptRoles(t *testing.T) {
	p := &Panel{Topic: "gophers", Roles: map[string]string{"claude": "security expert"}}
	first := p.turnPrompt("claude", nil, true)
	if !strings.Contains(first, `the security expert`) {
		t.Errorf("first prompt missing expert identity: %q", first)
	}
	next := p.turnPrompt("claude", []Turn{{Round: 1, Agent: "codex", Text: "hi"}}, false)
	if !strings.Contains(next, "Your expert role: security expert") {
		t.Errorf("continuation missing expert role: %q", next)
	}
	plain := (&Panel{Topic: "gophers"}).turnPrompt("codex", nil, true)
	if strings.Contains(plain, "expert") {
		t.Errorf("prompt without role should not mention expert: %q", plain)
	}
}

func TestRunRejectsUnknownRoleAgent(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	a, _ := twoFakes()
	p := &Panel{Agents: []adapter.Adapter{a}, Topic: "x", Rounds: 1,
		Roles: map[string]string{"ghost": "expert"}}
	if _, err := p.Run(context.Background()); err == nil {
		t.Error("expected error for role on unknown agent")
	}
}

func TestContextCancellation(t *testing.T) {
	t.Setenv("AGENT_RELAY_CONFIG_DIR", t.TempDir())
	a, _ := twoFakes()
	slow := &slowAdapter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &Panel{Agents: []adapter.Adapter{a, slow}, Topic: "x", Rounds: 2}
	if _, err := p.Run(ctx); !errors.Is(err, context.Canceled) && err == nil {
		t.Errorf("expected cancellation error, got %v", err)
	}
}

type slowAdapter struct{}

func (s *slowAdapter) Name() string                                  { return "slow" }
func (s *slowAdapter) Detect() error                                 { return nil }
func (s *slowAdapter) Command(req adapter.Request) ([]string, error) { return nil, nil }
func (s *slowAdapter) Send(ctx context.Context, req adapter.Request) (adapter.Reply, error) {
	<-ctx.Done()
	return adapter.Reply{}, ctx.Err()
}
