package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type OpenCode struct {
	bin string
}

func newOpenCode() *OpenCode { return &OpenCode{bin: "opencode"} }

func (o *OpenCode) Name() string { return "opencode" }

func (o *OpenCode) Detect() error {
	_, err := exec.LookPath(o.bin)
	return err
}

func (o *OpenCode) Command(req Request) ([]string, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("opencode: empty prompt")
	}
	args := []string{o.bin, "run", "--format", "json"}
	if req.Session != "" {
		args = append(args, "-s", req.Session)
	}
	if req.Model != "" {
		args = append(args, "-m", req.Model)
	}
	args = append(args, req.Prompt)
	return args, nil
}

type ocEvent struct {
	Type      string  `json:"type"`
	SessionID string  `json:"sessionID"`
	Part      *ocPart `json:"part"`
	Error     *ocErr  `json:"error"`
}

type ocPart struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

type ocErr struct {
	Name string `json:"name"`
	Data struct {
		Message string `json:"message"`
	} `json:"data"`
}

func (o *OpenCode) Send(ctx context.Context, req Request) (Reply, error) {
	start := time.Now()
	argv, err := o.Command(req)
	if err != nil {
		return Reply{}, err
	}
	stdout, stderr, err := run(ctx, req.Workdir, argv)

	var sessionID string
	var order []string
	parts := map[string]string{}
	var errMsgs []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev ocEvent
		if jsonErr := json.Unmarshal([]byte(line), &ev); jsonErr != nil {
			continue
		}
		if ev.SessionID != "" {
			sessionID = ev.SessionID
		}
		switch {
		case ev.Type == "text" && ev.Part != nil && ev.Part.ID != "":
			if _, seen := parts[ev.Part.ID]; !seen {
				order = append(order, ev.Part.ID)
			}
			parts[ev.Part.ID] = ev.Part.Text
		case ev.Type == "error" && ev.Error != nil:
			msg := ev.Error.Data.Message
			if msg == "" {
				msg = ev.Error.Name
			}
			errMsgs = append(errMsgs, msg)
		}
	}
	var b strings.Builder
	for i, id := range order {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(parts[id])
	}
	text := strings.TrimSpace(b.String())
	if err != nil {
		return Reply{}, fail("opencode", stderr, strings.Join(errMsgs, "; "), err)
	}
	if text == "" {
		return Reply{}, fail("opencode", stderr, strings.Join(errMsgs, "; "), fmt.Errorf("no text in output"))
	}
	return Reply{
		Agent:      o.Name(),
		Text:       text,
		SessionID:  sessionID,
		DurationMS: int(time.Since(start).Milliseconds()),
	}, nil
}

func init() { Register(newOpenCode()) }
