package adapter

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Request struct {
	Prompt  string
	Workdir string
	Session string
	Model   string
	Sandbox string
	Timeout time.Duration
}

type Reply struct {
	Agent      string `json:"agent"`
	Text       string `json:"text"`
	SessionID  string `json:"session_id"`
	DurationMS int    `json:"duration_ms"`
}

type Adapter interface {
	Name() string
	Detect() error
	Command(req Request) ([]string, error)
	Send(ctx context.Context, req Request) (Reply, error)
}

var registry = map[string]Adapter{}

func Register(a Adapter) {
	registry[a.Name()] = a
}

func Get(name string) (Adapter, error) {
	a, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q (available: %s)", name, strings.Join(Names(), ", "))
	}
	return a, nil
}

func Names() []string {
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func DetectAll() map[string]error {
	out := make(map[string]error, len(registry))
	for name, a := range registry {
		out[name] = a.Detect()
	}
	return out
}

func run(ctx context.Context, dir string, argv []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func fail(name, stderr, detail string, err error) error {
	msg := strings.TrimSpace(stderr)
	if detail != "" {
		if msg != "" {
			msg += ": " + detail
		} else {
			msg = detail
		}
	}
	if msg == "" && err != nil {
		msg = err.Error()
	}
	return fmt.Errorf("%s failed: %s", name, msg)
}
