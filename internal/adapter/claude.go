package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Claude struct {
	bin string
}

func newClaude() *Claude { return &Claude{bin: "claude"} }

func (c *Claude) Name() string { return "claude" }

func (c *Claude) Detect() error {
	_, err := exec.LookPath(c.bin)
	return err
}

func (c *Claude) Command(req Request) ([]string, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("claude: empty prompt")
	}
	args := []string{c.bin, "-p", req.Prompt, "--output-format", "json"}
	if req.Session != "" {
		args = append(args, "--resume", req.Session)
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	return args, nil
}

type claudeEvent struct {
	Type      string         `json:"type"`
	Subtype   string         `json:"subtype"`
	Result    string         `json:"result"`
	SessionID string         `json:"session_id"`
	IsError   bool           `json:"is_error"`
	Message   *claudeMessage `json:"message"`
}

type claudeMessage struct {
	Content []claudeBlock `json:"content"`
}

type claudeBlock struct {
	Type     string `json:"type"`
	Thinking string `json:"thinking"`
	Text     string `json:"text"`
}

func (c *Claude) Send(ctx context.Context, req Request) (Reply, error) {
	start := time.Now()
	argv, err := c.Command(req)
	if err != nil {
		return Reply{}, err
	}
	stdout, stderr, err := run(ctx, req.Workdir, argv)
	if err != nil {
		return Reply{}, fail("claude", stderr, "", err)
	}
	var events []claudeEvent
	if err := json.Unmarshal([]byte(stdout), &events); err != nil {
		// Modern `claude -p --output-format json` emits a single object,
		// older versions emit an array of events.
		var single claudeEvent
		if err2 := json.Unmarshal([]byte(stdout), &single); err2 != nil {
			return Reply{}, fmt.Errorf("claude: parse output: %w", err)
		}
		events = []claudeEvent{single}
	}
	var result *claudeEvent
	var thinking []string
	for i := range events {
		if events[i].Type == "result" {
			result = &events[i]
		}
		if events[i].Type == "assistant" && events[i].Message != nil {
			for _, b := range events[i].Message.Content {
				if b.Type == "thinking" && strings.TrimSpace(b.Thinking) != "" {
					thinking = append(thinking, strings.TrimSpace(b.Thinking))
				}
			}
		}
	}
	if result == nil {
		return Reply{}, fmt.Errorf("claude: no result event in output")
	}
	if result.IsError {
		return Reply{}, fmt.Errorf("claude: %s", strings.TrimSpace(result.Result))
	}
	return Reply{
		Agent:      c.Name(),
		Text:       result.Result,
		Thinking:   strings.Join(thinking, "\n\n"),
		SessionID:  result.SessionID,
		DurationMS: int(time.Since(start).Milliseconds()),
	}, nil
}

func init() { Register(newClaude()) }
