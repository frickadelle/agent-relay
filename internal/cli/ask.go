package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/frickadelle/agent-relay/internal/adapter"
	"github.com/frickadelle/agent-relay/internal/config"
	"github.com/frickadelle/agent-relay/internal/session"
	"github.com/spf13/cobra"
)

var (
	askWorkdir  string
	askModel    string
	askTimeout  time.Duration
	askJSON     bool
	askDryRun   bool
	askContinue string
	askNoSave   bool
)

var askCmd = &cobra.Command{
	Use:   "ask <agent> <prompt>",
	Short: "Send a prompt to a harness and print the reply",
	Long:  "Send a prompt to a coding harness (claude, codex, opencode) in non-interactive mode and print its reply. Every call is tracked in a relay session unless --no-save is set.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAsk(cmd, args[0], args[1])
	},
}

func runAsk(cmd *cobra.Command, agent, prompt string) error {
	a, err := adapter.Get(agent)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var sess *session.Session
	if askContinue != "" {
		sess, err = session.Load(askContinue)
		if err != nil {
			return err
		}
	} else if !askNoSave {
		sess = session.New(askWorkdir)
	}

	req := adapter.Request{
		Prompt:  prompt,
		Workdir: askWorkdir,
		Model:   askModel,
		Sandbox: cfg.Sandbox(agent),
	}
	if sess != nil {
		req.Session = sess.LastNativeID(agent)
		if req.Session != "" {
			fmt.Fprintf(os.Stderr, "resuming %s session %s\n", agent, req.Session)
		}
	}

	timeout := askTimeout
	if timeout == 0 {
		if timeout, err = cfg.Timeout(agent); err != nil {
			return err
		}
	}

	if askDryRun {
		argv, err := a.Command(req)
		if err != nil {
			return err
		}
		quoted := make([]string, len(argv))
		for i, arg := range argv {
			quoted[i] = fmt.Sprintf("%q", arg)
		}
		fmt.Println(strings.Join(quoted, " "))
		return nil
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	reply, err := a.Send(ctx, req)
	if err != nil {
		return err
	}

	if sess != nil {
		sess.AddTurn(agent, reply.SessionID, prompt, reply.Text)
		if err := session.Save(sess); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save session: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "relay session: %s (turn %d)\n", sess.ID, len(sess.Turns))
		}
	}

	if askJSON {
		return json.NewEncoder(os.Stdout).Encode(reply)
	}
	fmt.Println(reply.Text)
	return nil
}

func init() {
	askCmd.Flags().StringVar(&askWorkdir, "workdir", "", "working directory for the agent")
	askCmd.Flags().StringVar(&askModel, "model", "", "model override")
	askCmd.Flags().DurationVar(&askTimeout, "timeout", 0, "timeout (overrides config)")
	askCmd.Flags().BoolVar(&askJSON, "json", false, "print machine-readable reply")
	askCmd.Flags().BoolVar(&askDryRun, "dry-run", false, "print the command that would run")
	askCmd.Flags().StringVar(&askContinue, "continue", "", "continue this relay session")
	askCmd.Flags().BoolVar(&askNoSave, "no-save", false, "do not track this call in a session")
	rootCmd.AddCommand(askCmd)
}
