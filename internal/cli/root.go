package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:           "relay",
	Short:         "Connect CLI coding harnesses",
	Long:          "agent-relay connects CLI coding harnesses (Claude Code, Codex CLI, opencode) so they can delegate tasks to each other and discuss topics in round-robin panels.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
