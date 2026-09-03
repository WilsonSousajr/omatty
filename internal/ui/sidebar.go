// Package ui renders omatty. It is the only package that imports bubbletea.
package ui

import (
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Row is one line in the sidebar: a project header, or a session under it.
// Session is nil on a header row.
type Row struct {
	Project string
	Session *registry.Session
	Status  watcher.Status
}

// SidebarRows flattens state into display order: each project followed by
// its own sessions, projects in registration order. Sessions with no
// reported status render as idle.
//
//	rows := ui.SidebarRows(state, map[string]watcher.Status{"s2": watcher.StatusThinking})
func SidebarRows(st registry.State, status map[string]watcher.Status) []Row {
	rows := make([]Row, 0, len(st.Projects)+len(st.Sessions))
	for _, p := range st.Projects {
		rows = append(rows, Row{Project: p.Name})
		rows = append(rows, sessionRows(st, p.Name, status)...)
	}
	return rows
}

// sessionRows indexes st.Sessions rather than ranging by value, so each Row
// points at its own session instead of aliasing the loop variable.
func sessionRows(st registry.State, project string, status map[string]watcher.Status) []Row {
	var rows []Row
	for i := range st.Sessions {
		sess := &st.Sessions[i]
		if sess.Project != project {
			continue
		}
		s, ok := status[sess.ID]
		if !ok {
			s = watcher.StatusIdle
		}
		rows = append(rows, Row{Project: project, Session: sess, Status: s})
	}
	return rows
}

// Sidebar holds the row list and the cursor. The cursor only ever rests on
// a session row; project headers are labels, not targets.
type Sidebar struct {
	rows   []Row
	cursor int
}

// NewSidebar returns a Sidebar with the cursor on the first session row.
func NewSidebar(rows []Row) *Sidebar {
	s := &Sidebar{rows: rows, cursor: -1}
	s.MoveDown()
	return s
}

// Rows returns the rows in display order, for rendering.
func (s *Sidebar) Rows() []Row { return s.rows }

// SetRows replaces the rows while keeping the cursor on the same session if it
// is still present, so a live status update does not move the selection.
func (s *Sidebar) SetRows(rows []Row) {
	var selectedID string
	if sel, ok := s.Selected(); ok {
		selectedID = sel.Session.ID
	}
	s.rows = rows
	s.cursor = -1
	s.MoveDown()
	if selectedID == "" {
		return
	}
	for i, r := range rows {
		if r.Session != nil && r.Session.ID == selectedID {
			s.cursor = i
			return
		}
	}
}

// Selected returns the session row under the cursor. ok is false when the
// sidebar holds no sessions.
func (s *Sidebar) Selected() (Row, bool) {
	if s.cursor < 0 || s.cursor >= len(s.rows) {
		return Row{}, false
	}
	return s.rows[s.cursor], true
}

// MoveDown advances to the next session row, stopping at the last one.
func (s *Sidebar) MoveDown() { s.seek(1) }

// MoveUp retreats to the previous session row, stopping at the first one.
func (s *Sidebar) MoveUp() { s.seek(-1) }

// seek moves the cursor by step until it lands on a session row, leaving it
// where it was if there is none in that direction.
func (s *Sidebar) seek(step int) {
	for i := s.cursor + step; i >= 0 && i < len(s.rows); i += step {
		if s.rows[i].Session != nil {
			s.cursor = i
			return
		}
	}
}
