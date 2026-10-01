package roundtable

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/frickadelle/agent-relay/internal/activity"
	"github.com/frickadelle/agent-relay/internal/adapter"
	"github.com/frickadelle/agent-relay/internal/config"
)

const MaxRounds = 10

// DefaultMaxThinking caps reasoning context per turn (in chars) so one
// verbose thinker cannot blow up the prompt of every other agent.
const DefaultMaxThinking = 4000

type Turn struct {
	Round    int       `json:"round"`
	Agent    string    `json:"agent"`
	Text     string    `json:"text"`
	Thinking string    `json:"thinking,omitempty"`
	NativeID string    `json:"native_session,omitempty"`
	At       time.Time `json:"at"`
}

type Panel struct {
	Agents         []adapter.Adapter
	Topic          string
	Rounds         int
	Workdir        string
	MaxThinking    int
	Roles          map[string]string
	Sandbox        func(agent string) string
	Timeout        func(agent string) time.Duration
	OnTurn         func(Turn)
	TranscriptPath string
}

type transcriptWriter struct {
	f *os.File
}

func openTranscript(id string) (*transcriptWriter, error) {
	dir := filepath.Join(config.Dir(), "roundtables")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(dir, id+".jsonl"))
	if err != nil {
		return nil, err
	}
	return &transcriptWriter{f: f}, nil
}

func newPanelID() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("rt_%d", time.Now().UnixNano())
	}
	return "rt_" + hex.EncodeToString(b)
}

func (t *transcriptWriter) write(turn Turn) error {
	data, err := json.Marshal(turn)
	if err != nil {
		return err
	}
	if _, err := t.f.Write(append(data, '\n')); err != nil {
		return err
	}
	return t.f.Sync()
}

func (t *transcriptWriter) close() error {
	return t.f.Close()
}

func (p *Panel) Run(ctx context.Context) ([]Turn, error) {
	if len(p.Agents) == 0 {
		return nil, fmt.Errorf("roundtable needs at least one agent")
	}
	if p.Rounds < 1 || p.Rounds > MaxRounds {
		return nil, fmt.Errorf("rounds must be between 1 and %d", MaxRounds)
	}
	known := map[string]bool{}
	for _, a := range p.Agents {
		known[a.Name()] = true
	}
	for name := range p.Roles {
		if !known[name] {
			return nil, fmt.Errorf("role for unknown agent %q (panel has: %s)", name, strings.Join(agentNames(p.Agents), ", "))
		}
	}

	tw, err := openTranscript(newPanelID())
	if err != nil {
		return nil, err
	}
	defer tw.close()
	p.TranscriptPath = tw.f.Name()

	var transcript []Turn
	native := map[string]string{}
	seen := map[string]int{}

	for round := 1; round <= p.Rounds; round++ {
		for _, a := range p.Agents {
			if err := ctx.Err(); err != nil {
				return transcript, err
			}
			name := a.Name()
			req := adapter.Request{
				Prompt:  p.turnPrompt(name, transcript[seen[name]:], len(transcript) == 0),
				Workdir: p.Workdir,
				Session: native[name],
			}
			if p.Sandbox != nil {
				req.Sandbox = p.Sandbox(name)
			}
			turnCtx := ctx
			if p.Timeout != nil {
				var cancel context.CancelFunc
				turnCtx, cancel = context.WithTimeout(ctx, p.Timeout(name))
				defer cancel()
			}
			doneActive := activity.Track(activity.Active{
				Agent:   name,
				Workdir: p.Workdir,
				Topic:   p.Topic,
			})
			reply, err := a.Send(turnCtx, req)
			doneActive()
			if err != nil {
				return transcript, fmt.Errorf("round %d, %s: %w", round, name, err)
			}
			native[name] = reply.SessionID
			turn := Turn{Round: round, Agent: name, Text: reply.Text, Thinking: reply.Thinking, NativeID: reply.SessionID, At: time.Now()}
			transcript = append(transcript, turn)
			seen[name] = len(transcript)
			if err := tw.write(turn); err != nil {
				return transcript, fmt.Errorf("write transcript: %w", err)
			}
			if p.OnTurn != nil {
				p.OnTurn(turn)
			}
		}
	}
	return transcript, nil
}

func agentNames(agents []adapter.Adapter) []string {
	names := make([]string, 0, len(agents))
	for _, a := range agents {
		names = append(names, a.Name())
	}
	return names
}

func (p *Panel) turnPrompt(agent string, delta []Turn, first bool) string {
	var b strings.Builder
	role, expert := p.Roles[agent]
	identity := fmt.Sprintf("You are %q, one participant in a panel of AI coding agents.", agent)
	if expert {
		identity = fmt.Sprintf("You are %q, the %s in a panel of AI coding agents.", agent, role)
	}
	if first {
		fmt.Fprintf(&b, "%s\n\nTopic: %s\n\nRules:\n- Respond with your own contribution only.\n- Be concise; a few sentences unless code is needed.\n", identity, p.Topic)
		if expert {
			fmt.Fprintf(&b, "- Argue from your expert perspective as %s; defer to other experts outside your domain.\n", role)
		}
		if len(delta) > 0 {
			b.WriteString("\nTranscript so far:\n")
			p.renderTranscript(&b, delta)
			b.WriteString("\nAdd the first contribution from your perspective.")
		} else {
			b.WriteString("\nOpen the discussion with the first contribution from your perspective.")
		}
		return b.String()
	}
	fmt.Fprintf(&b, "Panel discussion continuation. Topic: %s\n", p.Topic)
	if expert {
		fmt.Fprintf(&b, "Your expert role: %s. Respond from that perspective.\n", role)
	}
	if len(delta) > 0 {
		b.WriteString("\nMessages from the other participants since your last turn:\n")
		p.renderTranscript(&b, delta)
		if p.MaxThinking > 0 {
			b.WriteString("\nLines marked (thinking) are reasoning context only; respond to the conclusions, do not repeat the thinking.\n")
		}
	} else {
		b.WriteString("\nNo new messages from the others since your last turn.\n")
	}
	b.WriteString("\nAdd your next contribution. Be concise.")
	return b.String()
}

func (p *Panel) renderTranscript(b *strings.Builder, turns []Turn) {
	for _, t := range turns {
		fmt.Fprintf(b, "[round %d] %s: %s\n", t.Round, t.Agent, t.Text)
		if p.MaxThinking > 0 && strings.TrimSpace(t.Thinking) != "" {
			fmt.Fprintf(b, "[round %d] %s (thinking): %s\n", t.Round, t.Agent, truncateThinking(t.Thinking, p.MaxThinking))
		}
	}
}

func truncateThinking(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max] + "\n…(thinking truncated)"
	}
	return s
}
