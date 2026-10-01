package activity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/frickadelle/agent-relay/internal/config"
)

// TTL bounds how long an entry counts as active. Crashed or killed calls
// never run their cleanup, so List prunes anything older than this.
const TTL = 30 * time.Minute

const previewLen = 200

// Active marks one running harness call so other chats can see who works on
// what right now. Entries are best-effort: written at call start, removed at
// call end, expired by TTL.
type Active struct {
	ID           string    `json:"id"`
	Agent        string    `json:"agent"`
	RelaySession string    `json:"relay_session,omitempty"`
	Room         string    `json:"room,omitempty"`
	Workdir      string    `json:"workdir,omitempty"`
	Topic        string    `json:"topic,omitempty"`
	Prompt       string    `json:"prompt,omitempty"`
	StartedAt    time.Time `json:"started_at"`
}

func Dir() string {
	return filepath.Join(config.Dir(), "active")
}

func path(id string) string {
	return filepath.Join(Dir(), id+".json")
}

// Track writes the entry and returns a cleanup func that removes it.
// Failures are silent: presence is advisory, never blocking.
func Track(a Active) func() {
	a.ID = fmt.Sprintf("a_%d_%d", os.Getpid(), time.Now().UnixNano())
	a.StartedAt = time.Now()
	if len(a.Prompt) > previewLen {
		a.Prompt = strings.TrimSpace(a.Prompt[:previewLen]) + "..."
	} else {
		a.Prompt = strings.TrimSpace(a.Prompt)
	}
	cleanup := func() { _ = os.Remove(path(a.ID)) }
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return cleanup
	}
	data, err := json.Marshal(a)
	if err != nil {
		return cleanup
	}
	if err := os.WriteFile(path(a.ID), data, 0o644); err != nil {
		return cleanup
	}
	return cleanup
}

// List returns fresh entries, newest first, and prunes expired ones.
func List() ([]Active, error) {
	entries, err := os.ReadDir(Dir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Active
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(Dir(), e.Name()))
		if err != nil {
			continue
		}
		var a Active
		if err := json.Unmarshal(data, &a); err != nil {
			continue
		}
		if time.Since(a.StartedAt) > TTL {
			_ = os.Remove(filepath.Join(Dir(), e.Name()))
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out, nil
}
