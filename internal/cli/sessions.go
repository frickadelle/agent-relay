package cli

import (
	"fmt"
	"strings"

	"github.com/frickadelle/agent-relay/internal/session"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage relay sessions",
}

var sessionsListWorkdir string
var sessionsListRoom string

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List relay sessions, newest first",
	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := session.ListFiltered(session.Filter{Room: sessionsListRoom, Workdir: sessionsListWorkdir})
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("no sessions yet")
			return nil
		}
		for _, s := range list {
			agents := map[string]bool{}
			for _, t := range s.Turns {
				agents[t.Agent] = true
			}
			names := make([]string, 0, len(agents))
			for a := range agents {
				names = append(names, a)
			}
			workdir := s.Workdir
			if workdir == "" {
				workdir = "-"
			}
			room := s.Room
			if room == "" {
				room = "-"
			}
			fmt.Printf("%s  %s  %d turns  [%s]  %s  room:%s\n",
				s.ID, s.CreatedAt.Format("2006-01-02 15:04"), len(s.Turns), strings.Join(names, ", "), workdir, room)
		}
		return nil
	},
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show the turns of a relay session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := session.Load(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("session %s\ncreated %s\n", s.ID, s.CreatedAt.Format("2006-01-02 15:04:05"))
		if s.Workdir != "" {
			fmt.Printf("workdir %s\n", s.Workdir)
		}
		if s.Room != "" {
			fmt.Printf("room %s\n", s.Room)
		}
		for i, t := range s.Turns {
			fmt.Printf("\n--- turn %d: %s at %s ---\n", i+1, t.Agent, t.At.Format("15:04:05"))
			if t.NativeID != "" {
				fmt.Printf("native session: %s\n", t.NativeID)
			}
			fmt.Printf("prompt: %s\n", t.PromptPreview)
			fmt.Printf("reply:  %s\n", t.ReplyPreview)
			if t.ThinkingPreview != "" {
				fmt.Printf("thinking: %s\n", t.ThinkingPreview)
			}
		}
		return nil
	},
}

var sessionsRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Delete a relay session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return session.Remove(args[0])
	},
}

func init() {
	sessionsListCmd.Flags().StringVar(&sessionsListWorkdir, "workdir", "", "only list sessions for this project directory (includes subdirectories)")
	sessionsListCmd.Flags().StringVar(&sessionsListRoom, "room", "", "only list sessions tagged with this room")
	sessionsCmd.AddCommand(sessionsListCmd, sessionsShowCmd, sessionsRmCmd)
	rootCmd.AddCommand(sessionsCmd)
}
