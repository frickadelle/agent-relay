package session

import (
	"path/filepath"
	"strings"

	"github.com/frickadelle/agent-relay/internal/config"
)

// Filter restricts sessions to a room and/or a directory (including children).
type Filter struct {
	Room    string
	Workdir string
}

func useSQLite() (bool, error) {
	cfg, err := config.Load()
	if err != nil {
		return false, err
	}
	return cfg.SessionStorage == "sqlite", nil
}

// Save writes a JSON snapshot, or atomically appends unsaved turns in SQLite mode.
// Previously persisted SQLite turns are immutable.
func Save(s *Session) error {
	sqlite, err := useSQLite()
	if err != nil {
		return err
	}
	if sqlite {
		return sqliteSave(s)
	}
	return jsonSave(s)
}

func Load(id string) (*Session, error) {
	sqlite, err := useSQLite()
	if err != nil {
		return nil, err
	}
	if sqlite {
		return sqliteLoad(id)
	}
	return jsonLoad(id)
}

func Remove(id string) error {
	sqlite, err := useSQLite()
	if err != nil {
		return err
	}
	if sqlite {
		return sqliteRemove(id)
	}
	return jsonRemove(id)
}

func List() ([]*Session, error) { return ListFiltered(Filter{}) }

func ListFiltered(filter Filter) ([]*Session, error) {
	sqlite, err := useSQLite()
	if err != nil {
		return nil, err
	}
	if sqlite {
		return sqliteList(filter, nil)
	}
	list, err := jsonList()
	if err != nil {
		return nil, err
	}
	var out []*Session
	for _, s := range list {
		if filter.Room != "" && s.Room != filter.Room {
			continue
		}
		if filter.Workdir != "" {
			wd, dir := filepath.Clean(s.Workdir), filepath.Clean(filter.Workdir)
			if wd != dir && !strings.HasPrefix(wd, dir+string(filepath.Separator)) {
				continue
			}
		}
		out = append(out, s)
	}
	return out, nil
}
