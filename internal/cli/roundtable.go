package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/frickadelle/agent-relay/internal/adapter"
	"github.com/frickadelle/agent-relay/internal/config"
	"github.com/frickadelle/agent-relay/internal/roundtable"
	"github.com/spf13/cobra"
)

var (
	rtAgents  string
	rtRounds  int
	rtWorkdir string
	rtTimeout time.Duration
)

var roundtableCmd = &cobra.Command{
	Use:   "roundtable <topic...>",
	Short: "Run a round-robin panel discussion between harnesses",
	Long:  "Run a panel discussion between two or more coding harnesses. Each agent sees the transcript of the others and continues its own native session across rounds. The transcript is stored as JSONL.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		names := strings.Split(rtAgents, ",")
		var agents []adapter.Adapter
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			a, err := adapter.Get(name)
			if err != nil {
				return err
			}
			if err := a.Detect(); err != nil {
				return fmt.Errorf("%s is not available: %w", name, err)
			}
			agents = append(agents, a)
		}
		if len(agents) < 2 {
			return fmt.Errorf("roundtable needs at least two agents, e.g. --agents claude,codex")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		panel := &roundtable.Panel{
			Agents:  agents,
			Topic:   strings.Join(args, " "),
			Rounds:  rtRounds,
			Workdir: rtWorkdir,
			Sandbox: cfg.Sandbox,
			Timeout: func(agent string) time.Duration {
				if rtTimeout > 0 {
					return rtTimeout
				}
				d, err := cfg.Timeout(agent)
				if err != nil {
					return 10 * time.Minute
				}
				return d
			},
			OnTurn: func(t roundtable.Turn) {
				fmt.Printf("\n=== round %d | %s ===\n%s\n", t.Round, t.Agent, t.Text)
			},
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()

		fmt.Fprintf(os.Stderr, "roundtable: %s | rounds: %d | topic: %s\n",
			strings.Join(names, ", "), rtRounds, panel.Topic)
		turns, err := panel.Run(ctx)
		if turns == nil && err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "\ntranscript: %s (%d turns)\n", panel.TranscriptPath, len(turns))
		return err
	},
}

func init() {
	roundtableCmd.Flags().StringVar(&rtAgents, "agents", "", "comma-separated agents, e.g. claude,codex,opencode")
	roundtableCmd.Flags().IntVar(&rtRounds, "rounds", 2, "rounds (each agent speaks once per round)")
	roundtableCmd.Flags().StringVar(&rtWorkdir, "workdir", "", "working directory for all agents")
	roundtableCmd.Flags().DurationVar(&rtTimeout, "timeout", 0, "per-turn timeout (overrides config)")
	_ = roundtableCmd.MarkFlagRequired("agents")
	rootCmd.AddCommand(roundtableCmd)
}
