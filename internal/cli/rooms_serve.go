package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/frickadelle/agent-relay/internal/activity"
	"github.com/frickadelle/agent-relay/internal/adapter"
	"github.com/frickadelle/agent-relay/internal/config"
	"github.com/frickadelle/agent-relay/internal/feed"
	"github.com/frickadelle/agent-relay/internal/session"
	"github.com/spf13/cobra"
)

var (
	serveRoom         string
	serveAgent        string
	serveAs           string
	serveInstructions string
	serveWorkdir      string
	serveModel        string
	serveTimeout      time.Duration
	serveIdleTimeout  time.Duration
	serveMaxTurns     int
	servePoll         time.Duration
)

type serveConfig struct {
	agent        adapter.Adapter
	room         string
	as           string
	instructions string
	workdir      string
	model        string
	sandbox      string
	timeout      time.Duration
	idleTimeout  time.Duration
	maxTurns     int
	poll         time.Duration
	onTurn       func(agent, text string)
}

// serveLoop waits for room messages and answers each with the same native
// thread. One relay session spans all turns, so the harness keeps its full
// context instead of cold-starting per message.
func serveLoop(ctx context.Context, cfg serveConfig) (*session.Session, error) {
	sess := session.New(cfg.workdir)
	sess.Room = cfg.room
	as := cfg.as
	if as == "" {
		as = cfg.agent.Name()
	}
	_, since, err := feed.Read(cfg.room, 0)
	if err != nil {
		return nil, err
	}
	turns := 0
	for {
		if cfg.maxTurns > 0 && turns >= cfg.maxTurns {
			return sess, nil
		}
		waitCtx := ctx
		var cancel context.CancelFunc
		if cfg.idleTimeout > 0 {
			waitCtx, cancel = context.WithTimeout(ctx, cfg.idleTimeout)
		}
		entries, cursor, err := feed.Wait(waitCtx, cfg.room, since, cfg.poll)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			if cfg.idleTimeout > 0 && ctx.Err() == nil {
				return sess, nil
			}
			return sess, err
		}
		since = cursor
		// Never answer own replies, or serve would talk to itself.
		var fresh []feed.Entry
		for _, e := range entries {
			if e.From != as {
				fresh = append(fresh, e)
			}
		}
		if len(fresh) == 0 {
			continue
		}
		prompt := formatInbox(fresh)
		if cfg.instructions != "" {
			prompt = cfg.instructions + "\n\n" + prompt
		}
		req := adapter.Request{
			Prompt:  prompt,
			Workdir: cfg.workdir,
			Model:   cfg.model,
			Sandbox: cfg.sandbox,
			Session: sess.LastNativeID(cfg.agent.Name()),
		}
		turnCtx := ctx
		turnCancel := context.CancelFunc(func() {})
		if cfg.timeout > 0 {
			turnCtx, turnCancel = context.WithTimeout(ctx, cfg.timeout)
		}
		done := activity.Track(activity.Active{
			Agent: cfg.agent.Name(), RelaySession: sess.ID,
			Room: cfg.room, Workdir: cfg.workdir, Prompt: prompt,
		})
		reply, err := cfg.agent.Send(turnCtx, req)
		done()
		turnCancel()
		if err != nil {
			return sess, err
		}
		sess.AddTurn(cfg.agent.Name(), reply.SessionID, prompt, reply.Text, reply.Thinking)
		if err := session.Save(sess); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save session: %v\n", err)
		}
		if _, err := feed.Post(cfg.room, as, reply.Text); err != nil {
			return sess, err
		}
		turns++
		if cfg.onTurn != nil {
			cfg.onTurn(cfg.agent.Name(), reply.Text)
		}
	}
}

func formatInbox(entries []feed.Entry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "[%s at %s]\n%s", e.From, e.At.Format("15:04:05"), e.Text)
	}
	return b.String()
}

var roomsServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Sit in a room and answer every new message",
	Long:  "Wait for room messages and answer each with the same native thread, posting replies back to the room. One relay session spans all turns. Ctrl+C or --idle-timeout stops serving.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := adapter.Get(serveAgent)
		if err != nil {
			return err
		}
		if err := a.Detect(); err != nil {
			return fmt.Errorf("%s is not available: %w", serveAgent, err)
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		timeout := serveTimeout
		if timeout == 0 {
			if timeout, err = cfg.Timeout(serveAgent); err != nil {
				return err
			}
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()
		fmt.Fprintf(os.Stderr, "serving room %s as %s (Ctrl+C stops)\n", serveRoom, serveAgent)
		sess, err := serveLoop(ctx, serveConfig{
			agent:        a,
			room:         serveRoom,
			as:           serveAs,
			instructions: serveInstructions,
			workdir:      serveWorkdir,
			model:        serveModel,
			sandbox:      cfg.Sandbox(serveAgent),
			timeout:      timeout,
			idleTimeout:  serveIdleTimeout,
			maxTurns:     serveMaxTurns,
			poll:         servePoll,
			onTurn: func(agent, text string) {
				fmt.Printf("\n=== %s ===\n%s\n", agent, text)
			},
		})
		if sess != nil {
			fmt.Fprintf(os.Stderr, "relay session: %s\n", sess.ID)
		}
		return err
	},
}

func init() {
	roomsServeCmd.Flags().StringVar(&serveRoom, "room", "", "room to serve")
	roomsServeCmd.Flags().StringVar(&serveAgent, "agent", "", "harness answering, e.g. claude")
	roomsServeCmd.Flags().StringVar(&serveAs, "as", "", "sender name for replies (defaults to agent)")
	roomsServeCmd.Flags().StringVar(&serveInstructions, "instructions", "", "prefix for every answer prompt")
	roomsServeCmd.Flags().StringVar(&serveWorkdir, "workdir", "", "working directory for the agent")
	roomsServeCmd.Flags().StringVar(&serveModel, "model", "", "model override")
	roomsServeCmd.Flags().DurationVar(&serveTimeout, "timeout", 0, "per-turn timeout (overrides config)")
	roomsServeCmd.Flags().DurationVar(&serveIdleTimeout, "idle-timeout", 0, "stop after this long without messages (0 serves forever)")
	roomsServeCmd.Flags().IntVar(&serveMaxTurns, "max-turns", 0, "stop after this many answers (0 unlimited)")
	roomsServeCmd.Flags().DurationVar(&servePoll, "poll", 2*time.Second, "feed check interval")
	_ = roomsServeCmd.MarkFlagRequired("room")
	_ = roomsServeCmd.MarkFlagRequired("agent")
	roomsCmd.AddCommand(roomsServeCmd)
}
