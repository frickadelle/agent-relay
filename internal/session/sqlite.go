package session

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/frickadelle/agent-relay/internal/config"
	_ "modernc.org/sqlite"
)

func newTurnID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("t_%d", time.Now().UnixNano())
	}
	return "t_" + hex.EncodeToString(b)
}

// Legacy turns get stable identities so repeated saves cannot duplicate them.
func ensureTurnIDs(s *Session) {
	for i := s.savedTurns; i < len(s.Turns); i++ {
		if s.Turns[i].ID != "" {
			continue
		}
		data, _ := json.Marshal(s.Turns[i])
		sum := sha256.Sum256(append([]byte(fmt.Sprintf("%s:%d:", s.ID, i)), data...))
		s.Turns[i].ID = "legacy_" + hex.EncodeToString(sum[:])
	}
}

func openSQLite() (*sql.DB, error) {
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(filepath.Join(config.Dir(), "sessions.db"))
	if err != nil {
		return nil, err
	}
	// Precreate with private permissions, without changing an existing database.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{"_pragma": {"busy_timeout(5000)", "foreign_keys(1)"}, "_txlock": {"immediate"}}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := initializeSQLite(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize session database: %w", err)
	}
	return db, nil
}

func initializeSQLite(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version == 1 {
		return nil
	}
	if version != 0 {
		return fmt.Errorf("unsupported session database version %d", version)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Recheck after taking the writer lock: another process may have imported.
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version == 1 {
		return tx.Commit()
	}
	if version != 0 {
		return fmt.Errorf("unsupported session database version %d", version)
	}
	_, err = tx.Exec(`
 CREATE TABLE sessions (
  id TEXT PRIMARY KEY, created_at TEXT NOT NULL, created_ns INTEGER NOT NULL,
  workdir TEXT NOT NULL, workdir_key TEXT NOT NULL, room TEXT NOT NULL
 );
 CREATE INDEX sessions_created ON sessions(created_ns DESC);
 CREATE INDEX sessions_room ON sessions(room, created_ns DESC);
 CREATE INDEX sessions_workdir ON sessions(workdir_key, created_ns DESC);
 CREATE TABLE turns (
  seq INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  id TEXT NOT NULL, agent TEXT NOT NULL, native_id TEXT NOT NULL,
  prompt TEXT NOT NULL, reply TEXT NOT NULL, thinking TEXT NOT NULL, at TEXT NOT NULL,
  UNIQUE(session_id, id)
 );`)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(Dir())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		s, err := jsonLoad(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return fmt.Errorf("import %s: %w", entry.Name(), err)
		}
		if s.ID != strings.TrimSuffix(entry.Name(), ".json") {
			return fmt.Errorf("import %s: session ID does not match filename", entry.Name())
		}
		if err := saveSQLiteTx(tx, s); err != nil {
			return fmt.Errorf("import %s: %w", entry.Name(), err)
		}
	}
	if _, err := tx.Exec("PRAGMA user_version = 1"); err != nil {
		return err
	}
	return tx.Commit()
}

func saveSQLiteTx(tx *sql.Tx, s *Session) error {
	_, err := tx.Exec(`INSERT INTO sessions(id, created_at, created_ns, workdir, workdir_key, room) VALUES(?,?,?,?,?,?)
 ON CONFLICT(id) DO UPDATE SET workdir=excluded.workdir, workdir_key=excluded.workdir_key, room=excluded.room`,
		s.ID, s.CreatedAt.Format(time.RFC3339Nano), s.CreatedAt.UnixNano(), s.Workdir, filepath.Clean(s.Workdir), s.Room)
	if err != nil {
		return err
	}
	ensureTurnIDs(s)
	for _, t := range s.Turns[s.savedTurns:] {
		_, err := tx.Exec(`INSERT INTO turns(session_id,id,agent,native_id,prompt,reply,thinking,at)
   VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(session_id,id) DO NOTHING`,
			s.ID, t.ID, t.Agent, t.NativeID, t.PromptPreview, t.ReplyPreview, t.ThinkingPreview, t.At.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
	}
	return nil
}

func sqliteSave(s *Session) error {
	db, err := openSQLite()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := saveSQLiteTx(tx, s); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.savedTurns = len(s.Turns)
	return nil
}

func sqliteLoad(id string) (*Session, error) {
	list, err := sqliteList(Filter{}, &id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("load session %q: %w", id, os.ErrNotExist)
	}
	return list[0], nil
}

func sqliteRemove(id string) error {
	db, err := openSQLite()
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("remove session %q: %w", id, os.ErrNotExist)
	}
	return nil
}

func sqliteList(filter Filter, id *string) ([]*Session, error) {
	db, err := openSQLite()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	where := []string{"1=1"}
	var args []any
	if id != nil {
		where = append(where, "s.id = ?")
		args = append(args, *id)
	}
	if filter.Room != "" {
		where = append(where, "s.room = ?")
		args = append(args, filter.Room)
	}
	if filter.Workdir != "" {
		dir := filepath.Clean(filter.Workdir)
		prefix := dir + string(filepath.Separator)
		// A lexical range uses the workdir index, including literal %, _ and \.
		where = append(where, "(s.workdir_key = ? OR (s.workdir_key >= ? AND s.workdir_key < ?))")
		args = append(args, dir, prefix, dir+string(filepath.Separator+1))
	}
	rows, err := db.Query(`SELECT s.id,s.created_at,s.workdir,s.room,t.seq,
 COALESCE(t.id,''),COALESCE(t.agent,''),COALESCE(t.native_id,''),COALESCE(t.prompt,''),
 COALESCE(t.reply,''),COALESCE(t.thinking,''),COALESCE(t.at,'')
 FROM sessions s LEFT JOIN turns t ON t.session_id=s.id WHERE `+strings.Join(where, " AND ")+`
 ORDER BY s.created_ns DESC,s.id,t.seq`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Session
	var current *Session
	for rows.Next() {
		var sid, created, wd, room, at string
		var seq sql.NullInt64
		var turn Turn
		if err := rows.Scan(&sid, &created, &wd, &room, &seq, &turn.ID, &turn.Agent, &turn.NativeID, &turn.PromptPreview, &turn.ReplyPreview, &turn.ThinkingPreview, &at); err != nil {
			return nil, err
		}
		if current == nil || current.ID != sid {
			parsed, err := time.Parse(time.RFC3339Nano, created)
			if err != nil {
				return nil, err
			}
			current = &Session{ID: sid, CreatedAt: parsed, Workdir: wd, Room: room}
			out = append(out, current)
		}
		if seq.Valid {
			turn.At, err = time.Parse(time.RFC3339Nano, at)
			if err != nil {
				return nil, err
			}
			current.Turns = append(current.Turns, turn)
			current.savedTurns++
		}
	}
	return out, rows.Err()
}
