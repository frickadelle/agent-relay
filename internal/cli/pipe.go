package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var pipeCmd = &cobra.Command{
	Use:   "pipe <agent> [instructions...]",
	Short: "Pipe stdin to a harness",
	Long:  "Read stdin and send it to a harness, optionally preceded by instructions. Example: git diff | relay pipe codex \"review this diff\"",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		stat, err := os.Stdin.Stat()
		if err != nil {
			return err
		}
		if stat.Mode()&os.ModeCharDevice != 0 {
			return fmt.Errorf("pipe requires stdin input, e.g. git diff | relay pipe codex")
		}
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		stdin := strings.TrimSpace(string(input))
		if stdin == "" {
			return fmt.Errorf("stdin is empty")
		}
		prompt := stdin
		if len(args) > 1 {
			prompt = strings.Join(args[1:], " ") + "\n\n" + stdin
		}
		return runAsk(cmd, args[0], prompt)
	},
}

func init() {
	pipeCmd.Flags().AddFlagSet(askCmd.Flags())
	rootCmd.AddCommand(pipeCmd)
}
