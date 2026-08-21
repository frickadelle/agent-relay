package adapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubBin(t *testing.T, name string, fixture string, exitCode int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	script := fmt.Sprintf("#!/bin/sh\ncat <<'RELAY_FIXTURE_EOF'\n%s\nRELAY_FIXTURE_EOF\nexit %d\n", fixture, exitCode)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestClaudeSend(t *testing.T) {
	c := &Claude{bin: stubBin(t, "claude", readFixture(t, "claude_ok.json"), 0)}
	reply, err := c.Send(context.Background(), Request{Prompt: "hi"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if reply.Text != "ok" {
		t.Errorf("Text = %q, want ok", reply.Text)
	}
	if reply.SessionID != "79b79f26-9804-4fda-98d2-74497bdb3a76" {
		t.Errorf("SessionID = %q", reply.SessionID)
	}
	if reply.Agent != "claude" {
		t.Errorf("Agent = %q", reply.Agent)
	}
}

func TestClaudeSendErrorResult(t *testing.T) {
	c := &Claude{bin: stubBin(t, "claude", readFixture(t, "claude_error.json"), 0)}
	if _, err := c.Send(context.Background(), Request{Prompt: "hi"}); err == nil {
		t.Fatal("expected error for is_error result")
	} else if !strings.Contains(err.Error(), "Permission denied") {
		t.Errorf("error should contain result text, got: %v", err)
	}
}

func TestClaudeCommand(t *testing.T) {
	c := newClaude()
	argv, err := c.Command(Request{Prompt: "do it", Session: "s1", Model: "haiku"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"-p do it", "--output-format json", "--resume s1", "--model haiku"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q: %v", want, argv)
		}
	}
	if _, err := c.Command(Request{Prompt: "  "}); err == nil {
		t.Error("expected error for empty prompt")
	}
}
