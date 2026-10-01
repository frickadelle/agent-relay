package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/frickadelle/agent-relay/internal/activity"
	"github.com/frickadelle/agent-relay/internal/session"
	"github.com/spf13/cobra"
)

// recentLimit bounds the recent-sessions section so status stays scannable.
const recentLimit = 5

var (
	statusRoom    string
	statusWorkdir string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show who works on what right now",
	Long:  "Show currently running harness calls plus recent sessions, optionally filtered by room or project directory. Check this before starting file work to avoid overlapping with another chat.",
	RunE: func(cmd *cobra.Command, args []string) error {
		active, err := activity.List()
		if err != nil {
			return err
		}
		active = filterActive(active, statusRoom, statusWorkdir)
		if len(active) == 0 {
			fmt.Println("nothing active right now")
		} else {
			fmt.Println("active now:")
			for _, a := range active {
				fmt.Printf("  %s  %s  %s  since %s  %q\n",
					a.Agent, describeScope(a.Room, a.Workdir), describeWhat(a.Topic, a.Prompt),
					a.StartedAt.Format("15:04"), shorten(a.RelaySession))
			}
		}

		list, err := session.List()
		if err != nil {
			return err
		}
		if statusRoom != "" {
			list = filterByRoom(list, statusRoom)
		}
		if statusWorkdir != "" {
			list = filterByWorkdir(list, statusWorkdir)
		}
		if len(list) > recentLimit {
			list = list[:recentLimit]
		}
		if len(list) == 0 {
			return nil
		}
		fmt.Println("recent sessions:")
		for _, s := range list {
			agents := map[string]bool{}
			for _, t := range s.Turns {
				agents[t.Agent] = true
			}
			names := make([]string, 0, len(agents))
			for a := range agents {
				names = append(names, a)
			}
			fmt.Printf("  %s  %d turns  [%s]  %s\n",
				s.ID, len(s.Turns), strings.Join(names, ", "), describeScope(s.Room, s.Workdir))
		}
		return nil
	},
}

func filterActive(list []activity.Active, room, workdir string) []activity.Active {
	var out []activity.Active
	for _, a := range list {
		if room != "" && a.Room != room {
			continue
		}
		if workdir != "" && !underDir(a.Workdir, workdir) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func underDir(wd, dir string) bool {
	wd, dir = filepath.Clean(wd), filepath.Clean(dir)
	return wd == dir || strings.HasPrefix(wd, dir+string(filepath.Separator))
}

func describeScope(room, workdir string) string {
	var parts []string
	if room != "" {
		parts = append(parts, "room:"+room)
	}
	if workdir != "" {
		parts = append(parts, "dir:"+workdir)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func describeWhat(topic, prompt string) string {
	if topic != "" {
		return "topic:" + topic
	}
	if prompt != "" {
		return prompt
	}
	return "-"
}

func shorten(id string) string {
	if id == "" {
		return "-"
	}
	return "session:" + id
}

func init() {
	statusCmd.Flags().StringVar(&statusRoom, "room", "", "only show activity for this room")
	statusCmd.Flags().StringVar(&statusWorkdir, "workdir", "", "only show activity for this project directory (includes subdirectories)")
	rootCmd.AddCommand(statusCmd)
}
