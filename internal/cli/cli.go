// Package cli is omatty's second driving adapter (ADR 0001, "The second
// driving adapter"; migration step 7.1, #653): one-shot reads through the
// same services the TUI uses, written as JSON for a script. It exists to prove
// the rule as much as to be used - it compiles and passes with no bubbletea in
// its import graph, so the core is UI-free. The JSON shape is not frozen
// below 1.0, like state.json and the key table.
//
//	err := cli.Sessions(ctx, os.Stdout, store)
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/domain/status"
)

// Store is the slice of the session service's StateStore these reads need.
type Store interface {
	Load(ctx context.Context) (session.State, error)
}

// StatusReader is a session's state as its transcript implies, read once.
// cmd builds it from service/status's Read over infra/transcript's reader.
type StatusReader func(sess session.Session) status.SessionState

// sessionJSON is one row of `omatty sessions --json`.
type sessionJSON struct {
	ID           string `json:"id"`
	Project      string `json:"project"`
	Title        string `json:"title"`
	Dir          string `json:"dir"`
	Branch       string `json:"branch,omitempty"`
	Worktree     bool   `json:"worktree"`
	Conversation string `json:"conversation,omitempty"`
}

// statusJSON is one row of `omatty status --json`.
type statusJSON struct {
	ID      string        `json:"id"`
	Project string        `json:"project"`
	Title   string        `json:"title"`
	Status  status.Status `json:"status"`
	Since   *time.Time    `json:"since,omitempty"`
	Tokens  tokensJSON    `json:"tokens"`
}

// tokensJSON is a session's cumulative usage, named as the API names it.
type tokensJSON struct {
	In         int `json:"input"`
	Out        int `json:"output"`
	CacheRead  int `json:"cache_read"`
	CacheWrite int `json:"cache_write"`
}

// Sessions writes every registered session as a JSON array.
//
//	err := cli.Sessions(ctx, os.Stdout, store)
func Sessions(ctx context.Context, w io.Writer, store Store) error {
	st, err := store.Load(ctx)
	if err != nil {
		return err
	}
	rows := make([]sessionJSON, 0, len(st.Sessions))
	for _, s := range st.Sessions {
		rows = append(rows, sessionJSON{ID: s.ID, Project: s.Project, Title: s.Title, Dir: s.Dir,
			Branch: s.Branch, Worktree: s.Worktree, Conversation: s.Conversation})
	}
	return write(w, rows)
}

// Status writes every session's status, as read reports it, as a JSON array.
// A session with no status yet reads idle, as its card does.
//
//	err := cli.Status(ctx, os.Stdout, store, read)
func Status(ctx context.Context, w io.Writer, store Store, read StatusReader) error {
	st, err := store.Load(ctx)
	if err != nil {
		return err
	}
	rows := make([]statusJSON, 0, len(st.Sessions))
	for _, s := range st.Sessions {
		rows = append(rows, statusRow(s, read(s)))
	}
	return write(w, rows)
}

// statusRow is one session's row.
func statusRow(s session.Session, st status.SessionState) statusJSON {
	row := statusJSON{ID: s.ID, Project: s.Project, Title: s.Title, Status: st.Status,
		Tokens: tokensJSON{In: st.Tokens.In, Out: st.Tokens.Out, CacheRead: st.Tokens.CacheRead, CacheWrite: st.Tokens.CacheWrite}}
	if row.Status == "" {
		row.Status = status.StatusIdle
	}
	if !st.At.IsZero() {
		at := st.At.UTC()
		row.Since = &at
	}
	return row
}

// write is rows as indented JSON and a newline.
func write(w io.Writer, rows any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		return fmt.Errorf("cli: writing JSON: %w", err)
	}
	return nil
}
