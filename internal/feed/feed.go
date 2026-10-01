package feed

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/frickadelle/agent-relay/internal/config"
)

// validRoom keeps room names filesystem-safe (no path traversal).
var validRoom = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// Entry is one message in a room feed.
type Entry struct {
	Seq  int       `json:"seq"`
	At   time.Time `json:"at"`
	From string    `json:"from"`
	Text string    `json:"text"`
}

func Dir() string {
	return filepath.Join(config.Dir(), "rooms")
}

func path(room string) (string, error) {
	if !validRoom.MatchString(room) {
		return "", fmt.Errorf("bad room name %q (letters, digits, -, _)", room)
	}
	return filepath.Join(Dir(), room+".jsonl"), nil
}

// Post appends a message to the room feed and returns it with its sequence
// number. Seq starts at 1 and counts feed lines.
func Post(room, from, text string) (Entry, error) {
	if strings.TrimSpace(from) == "" {
		return Entry{}, fmt.Errorf("rooms post needs --from")
	}
	if strings.TrimSpace(text) == "" {
		return Entry{}, fmt.Errorf("rooms post needs message text")
	}
	p, err := path(room)
	if err != nil {
		return Entry{}, err
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return Entry{}, err
	}
	_, cursor, err := Read(room, 0)
	if err != nil {
		return Entry{}, err
	}
	e := Entry{Seq: cursor + 1, At: time.Now(), From: from, Text: text}
	data, err := json.Marshal(e)
	if err != nil {
		return Entry{}, err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Entry{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return Entry{}, err
	}
	return e, f.Sync()
}

// Read returns entries with Seq > since plus the current cursor (total line
// count). Malformed lines are skipped.
func Read(room string, since int) ([]Entry, int, error) {
	p, err := path(room)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(p)
	if os.IsNotExist(err) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	var out []Entry
	cursor := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		cursor++
		if cursor <= since {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		e.Seq = cursor
		out = append(out, e)
	}
	return out, cursor, sc.Err()
}

// Wait blocks until the feed holds entries beyond since, then returns them.
// It returns immediately if such entries already exist. The context controls
// timeout and cancellation (Ctrl+C).
func Wait(ctx context.Context, room string, since int, poll time.Duration) ([]Entry, int, error) {
	if poll <= 0 {
		poll = 2 * time.Second
	}
	if _, err := path(room); err != nil {
		return nil, since, err
	}
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		entries, cursor, err := Read(room, since)
		if err != nil {
			return nil, since, err
		}
		if len(entries) > 0 {
			return entries, cursor, nil
		}
		select {
		case <-ctx.Done():
			return nil, cursor, ctx.Err()
		case <-t.C:
		}
	}
}
