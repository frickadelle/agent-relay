package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/frickadelle/agent-relay/internal/config"
)

const previewLen = 300

type Turn struct {
	Agent           string    `json:"agent"`
	NativeID        string    `json:"native_session,omitempty"`
	PromptPreview   string    `json:"prompt_preview,omitempty"`
	ReplyPreview    string    `json:"reply_preview,omitempty"`
	ThinkingPreview string    `json:"thinking_preview,omitempty"`
	At              time.Time `json:"at"`
}

type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Workdir   string    `json:"workdir,omitempty"`
	Room      string    `json:"room,omitempty"`
	Turns     []Turn    `json:"turns"`
}

func Dir() string {
	return filepath.Join(config.Dir(), "sessions")
}

func NewID() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("r_%s_%d", time.Now().Format("20060102"), time.Now().UnixNano()%1e6)
	}
	return fmt.Sprintf("r_%s_%s", time.Now().Format("20060102"), hex.EncodeToString(b))
}

func New(workdir string) *Session {
	return &Session{ID: NewID(), CreatedAt: time.Now(), Workdir: workdir}
}

func path(id string) string {
	return filepath.Join(Dir(), id+".json")
}

func Save(s *Session) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path(s.ID), data, 0o644)
}

func Load(id string) (*Session, error) {
	data, err := os.ReadFile(path(id))
	if err != nil {
		return nil, fmt.Errorf("load session %q: %w", id, err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse session %q: %w", id, err)
	}
	return &s, nil
}

func Remove(id string) error {
	if err := os.Remove(path(id)); err != nil {
		return fmt.Errorf("remove session %q: %w", id, err)
	}
	return nil
}

func List() ([]*Session, error) {
	entries, err := os.ReadDir(Dir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := Load(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > previewLen {
		return s[:previewLen] + "..."
	}
	return s
}

func (s *Session) LastNativeID(agent string) string {
	for i := len(s.Turns) - 1; i >= 0; i-- {
		if s.Turns[i].Agent == agent && s.Turns[i].NativeID != "" {
			return s.Turns[i].NativeID
		}
	}
	return ""
}

func (s *Session) AddTurn(agent, nativeID, prompt, reply, thinking string) {
	s.Turns = append(s.Turns, Turn{
		Agent:           agent,
		NativeID:        nativeID,
		PromptPreview:   truncate(prompt),
		ReplyPreview:    truncate(reply),
		ThinkingPreview: truncate(thinking),
		At:              time.Now(),
	})
}
