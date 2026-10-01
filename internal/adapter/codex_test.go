package adapter

import (
	"context"
	"strings"
	"testing"
)

func TestCodexSend(t *testing.T) {
	c := &Codex{bin: stubBin(t, "codex", readFixture(t, "codex_ok.jsonl"), 0)}
	reply, err := c.Send(context.Background(), Request{Prompt: "hi"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if reply.Text != "ok" {
		t.Errorf("Text = %q, want ok", reply.Text)
	}
	if reply.SessionID != "01a0238e-6918-7013-8466-8c071db72b40" {
		t.Errorf("SessionID = %q", reply.SessionID)
	}
}

func TestCodexSendFailure(t *testing.T) {
	c := &Codex{bin: stubBin(t, "codex", readFixture(t, "codex_error.jsonl"), 1)}
	_, err := c.Send(context.Background(), Request{Prompt: "hi"})
	if err == nil {
		t.Fatal("expected error for failing run")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should carry item message, got: %v", err)
	}
}

func TestCodexSendReasoning(t *testing.T) {
	fixture := "{\"type\":\"thread.started\",\"thread_id\":\"thr_1\"}\n" +
		"{\"type\":\"item.completed\",\"item\":{\"id\":\"item_0\",\"type\":\"reasoning\",\"text\":\"check plan first\"}}\n" +
		"{\"type\":\"item.completed\",\"item\":{\"id\":\"item_1\",\"type\":\"reasoning\",\"text\":\"then edit\"}}\n" +
		"{\"type\":\"item.completed\",\"item\":{\"id\":\"item_2\",\"type\":\"agent_message\",\"text\":\"ok\"}}\n"
	c := &Codex{bin: stubBin(t, "codex", fixture, 0)}
	reply, err := c.Send(context.Background(), Request{Prompt: "hi"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if reply.Text != "ok" {
		t.Errorf("Text = %q, want ok", reply.Text)
	}
	if !strings.Contains(reply.Thinking, "check plan first") || !strings.Contains(reply.Thinking, "then edit") {
		t.Errorf("Thinking = %q, want both reasoning items", reply.Thinking)
	}
}

func TestCodexCommand(t *testing.T) {
	c := newCodex()
	argv, err := c.Command(Request{Prompt: "go", Session: "t1", Model: "gpt-5.2", Sandbox: "read-only"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"exec --json", "-s read-only", "-m gpt-5.2", "resume t1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q: %v", want, argv)
		}
	}
}
