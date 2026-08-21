package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/frickadelle/agent-relay/internal/config"
	"github.com/spf13/cobra"
)

var harnessBins = []string{"claude", "codex", "opencode"}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check installed harnesses and configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		ok := true
		for _, bin := range harnessBins {
			path, err := exec.LookPath(bin)
			if err != nil {
				fmt.Printf("  x %-10s not found in PATH\n", bin)
				ok = false
				continue
			}
			fmt.Printf("  + %-10s %s\n", bin, path)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("  x config    %s: %v\n", config.Path(), err)
			ok = false
		} else {
			if _, statErr := os.Stat(config.Path()); statErr == nil {
				fmt.Printf("  + config    %s (timeout=%s)\n", config.Path(), cfg.DefaultTimeout)
			} else {
				fmt.Printf("  . config    %s does not exist yet, using defaults\n", config.Path())
			}
		}

		if !ok {
			return fmt.Errorf("doctor found problems")
		}
		return nil
	},
}
