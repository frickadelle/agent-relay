package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/frickadelle/agent-relay/internal/feed"
	"github.com/frickadelle/agent-relay/internal/session"
	"github.com/spf13/cobra"
)

type roomSummary struct {
	Name       string
	Sessions   int
	LastActive time.Time
}

// summarizeRooms groups sessions by room tag, newest activity first.
// Sessions without a room tag are skipped.
func summarizeRooms(list []*session.Session) []roomSummary {
	byName := map[string]*roomSummary{}
	for _, s := range list {
		if s.Room == "" {
			continue
		}
		r, ok := byName[s.Room]
		if !ok {
			r = &roomSummary{Name: s.Room}
			byName[s.Room] = r
		}
		r.Sessions++
		if last := lastActive(s); last.After(r.LastActive) {
			r.LastActive = last
		}
	}
	out := make([]roomSummary, 0, len(byName))
	for _, r := range byName {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastActive.After(out[j].LastActive)
	})
	return out
}

func lastActive(s *session.Session) time.Time {
	latest := s.CreatedAt
	for _, t := range s.Turns {
		if t.At.After(latest) {
			latest = t.At
		}
	}
	return latest
}

var roomsCmd = &cobra.Command{
	Use:   "rooms",
	Short: "Manage topic rooms",
}

var roomsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List topic rooms, most recently active first",
	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := session.List()
		if err != nil {
			return err
		}
		rooms := summarizeRooms(list)
		if len(rooms) == 0 {
			fmt.Println("no rooms yet (tag a session with --room)")
			return nil
		}
		for _, r := range rooms {
			fmt.Printf("%s  %d sessions  last active %s\n",
				r.Name, r.Sessions, r.LastActive.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var (
	roomsPostRoom    string
	roomsPostFrom    string
	roomsWaitRoom    string
	roomsWaitSince   int
	roomsWaitTimeout time.Duration
	roomsWaitPoll    time.Duration
	roomsWaitJSON    bool
)

var roomsPostCmd = &cobra.Command{
	Use:   "post [message...]",
	Short: "Post a message to a room feed",
	Long:  "Append a message to a room feed so waiting chats can read the full text. Message comes from args or stdin, e.g. echo hi | relay rooms post --room billing --from codex",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		text := strings.Join(args, " ")
		if text == "" && !isCharDevice(os.Stdin) {
			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			text = strings.TrimSpace(string(input))
		}
		e, err := feed.Post(roomsPostRoom, roomsPostFrom, text)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "posted #%d to room %s\n", e.Seq, roomsPostRoom)
		return nil
	},
}

func isCharDevice(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

var roomsWaitCmd = &cobra.Command{
	Use:   "wait",
	Short: "Wait for new room messages, then print them",
	Long:  "Block until the room feed holds messages beyond --since (default: everything posted from now on), print them, and exit. Ctrl+C or --timeout stops waiting.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()
		if roomsWaitTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, roomsWaitTimeout)
			defer cancel()
		}
		since := roomsWaitSince
		if !cmd.Flags().Changed("since") {
			_, since, _ = feed.Read(roomsWaitRoom, 0)
		}
		entries, cursor, err := feed.Wait(ctx, roomsWaitRoom, since, roomsWaitPoll)
		if err != nil {
			return err
		}
		if roomsWaitJSON {
			return json.NewEncoder(os.Stdout).Encode(entries)
		}
		for _, e := range entries {
			fmt.Printf("[#%d] %s at %s\n%s\n", e.Seq, e.From, e.At.Format("15:04:05"), e.Text)
		}
		fmt.Fprintf(os.Stderr, "cursor: %d\n", cursor)
		return nil
	},
}

func init() {
	roomsPostCmd.Flags().StringVar(&roomsPostRoom, "room", "", "room to post to")
	roomsPostCmd.Flags().StringVar(&roomsPostFrom, "from", "", "sender name, e.g. codex")
	_ = roomsPostCmd.MarkFlagRequired("room")
	_ = roomsPostCmd.MarkFlagRequired("from")
	roomsWaitCmd.Flags().StringVar(&roomsWaitRoom, "room", "", "room to wait on")
	roomsWaitCmd.Flags().IntVar(&roomsWaitSince, "since", 0, "print messages after this sequence number (default: only new messages)")
	roomsWaitCmd.Flags().DurationVar(&roomsWaitTimeout, "timeout", 0, "stop waiting after this long (0 waits forever)")
	roomsWaitCmd.Flags().DurationVar(&roomsWaitPoll, "poll", 2*time.Second, "feed check interval")
	roomsWaitCmd.Flags().BoolVar(&roomsWaitJSON, "json", false, "print messages as JSON")
	_ = roomsWaitCmd.MarkFlagRequired("room")
	roomsCmd.AddCommand(roomsListCmd, roomsPostCmd, roomsWaitCmd)
	rootCmd.AddCommand(roomsCmd)
}
