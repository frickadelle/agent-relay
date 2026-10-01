package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Codex struct {
	bin string
}

func newCodex() *Codex { return &Codex{bin: "codex"} }

func (c *Codex) Name() string { return "codex" }

func (c *Codex) Detect() error {
	_, err := exec.LookPath(c.bin)
	return err
}

func (c *Codex) Command(req Request) ([]string, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("codex: empty prompt")
	}
	args := []string{c.bin, "exec", "--json", "--skip-git-repo-check"}
	if req.Sandbox != "" {
		args = append(args, "-s", req.Sandbox)
	}
	if req.Model != "" {
		args = append(args, "-m", req.Model)
	}
	if req.Session != "" {
		args = append(args, "resume", req.Session)
	}
	args = append(args, req.Prompt)
	return args, nil
}

type codexEvent struct {
	Type     string     `json:"type"`
	ThreadID string     `json:"thread_id"`
	Item     *codexItem `json:"item"`
}

type codexItem struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Text    string `json:"text"`
	Message string `json:"message"`
}

func (c *Codex) Send(ctx context.Context, req Request) (Reply, error) {
	start := time.Now()
	argv, err := c.Command(req)
	if err != nil {
		return Reply{}, err
	}
	stdout, stderr, err := run(ctx, req.Workdir, argv)

	var sessionID string
	var text string
	var thinking []string
	var errMsgs []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev codexEvent
		if jsonErr := json.Unmarshal([]byte(line), &ev); jsonErr != nil {
			continue
		}
		switch {
		case ev.Type == "thread.started" && ev.ThreadID != "":
			sessionID = ev.ThreadID
		case ev.Type == "item.completed" && ev.Item != nil && ev.Item.Type == "agent_message":
			text = ev.Item.Text
		case ev.Type == "item.completed" && ev.Item != nil && ev.Item.Type == "reasoning":
			if strings.TrimSpace(ev.Item.Text) != "" {
				thinking = append(thinking, strings.TrimSpace(ev.Item.Text))
			}
		case ev.Type == "item.completed" && ev.Item != nil && ev.Item.Type == "error":
			errMsgs = append(errMsgs, ev.Item.Message)
		}
	}
	if err != nil {
		return Reply{}, fail("codex", stderr, strings.Join(errMsgs, "; "), err)
	}
	if text == "" {
		return Reply{}, fail("codex", stderr, strings.Join(errMsgs, "; "), fmt.Errorf("no agent message in output"))
	}
	return Reply{
		Agent:      c.Name(),
		Text:       text,
		Thinking:   strings.Join(thinking, "\n\n"),
		SessionID:  sessionID,
		DurationMS: int(time.Since(start).Milliseconds()),
	}, nil
}

func init() { Register(newCodex()) }
