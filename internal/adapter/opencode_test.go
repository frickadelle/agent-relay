package adapter

import (
	"context"
	"strings"
	"testing"
)

func TestOpenCodeSend(t *testing.T) {
	o := &OpenCode{bin: stubBin(t, "opencode", readFixture(t, "opencode_ok.jsonl"), 0)}
	reply, err := o.Send(context.Background(), Request{Prompt: "hi"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if reply.Text != "ok" {
		t.Errorf("Text = %q, want ok", reply.Text)
	}
	if reply.SessionID != "ses_fdc6f7abdffeSbk3pAS2o5AInj" {
		t.Errorf("SessionID = %q", reply.SessionID)
	}
}

func TestOpenCodeSendErrorEvent(t *testing.T) {
	o := &OpenCode{bin: stubBin(t, "opencode", readFixture(t, "opencode_error.jsonl"), 1)}
	_, err := o.Send(context.Background(), Request{Prompt: "hi"})
	if err == nil {
		t.Fatal("expected error for APIError event")
	}
	if !strings.Contains(err.Error(), "No endpoints found") {
		t.Errorf("error should carry API message, got: %v", err)
	}
}

func TestOpenCodeCommand(t *testing.T) {
	o := newOpenCode()
	argv, err := o.Command(Request{Prompt: "go", Session: "ses_1", Model: "opencode/hy3-free"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"run --format json", "-s ses_1", "-m opencode/hy3-free"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q: %v", want, argv)
		}
	}
}
