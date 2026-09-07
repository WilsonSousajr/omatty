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
	offset int // first row drawn; recomputed by Window each frame (#129)
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
	s.SelectByID(selectedID)
}

// SelectByID puts the cursor on a session wherever it is, and reports whether
// it was found. Unlike MoveUp and MoveDown it can jump in either direction and
// across projects, which is what the switcher needs (#42).
//
//	if sb.SelectByID(id) { /* the cursor is on id */ }
func (s *Sidebar) SelectByID(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	for i, r := range s.rows {
		if r.Session != nil && r.Session.ID == sessionID {
			s.cursor = i
			return true
		}
	}
	return false
}

// Selected returns the session row under the cursor. ok is false when the
// sidebar holds no sessions.
func (s *Sidebar) Selected() (Row, bool) {
	if s.cursor < 0 || s.cursor >= len(s.rows) {
		return Row{}, false
	}
	return s.rows[s.cursor], true
}

// Window returns the rows to draw when only rows lines fit, keeping the
// cursor inside them. The offset is recomputed from the cursor on every call
// rather than trusted from the last move, because a resize can shrink the
// pane after the cursor last moved - the same reason renderTree does it (#129).
//
//	for _, row := range sb.Window(paneRows - 1) { ... }
func (s *Sidebar) Window(rows int) []Row {
	s.offset = s.revealHeader(ScrollOffset(max(s.cursor, 0), s.offset, rows))
	end := min(s.offset+rows, len(s.rows))
	if s.offset >= end {
		return nil
	}
	return s.rows[s.offset:end]
}

// revealHeader scrolls one more row when the cursor sits on the top line
// and the row above it is its own project's header, so moving up onto a
// project's first session shows the project's name too rather than an
// unlabelled row at the top of the pane (#129).
func (s *Sidebar) revealHeader(offset int) int {
	if offset > 0 && offset == s.cursor && s.rows[offset-1].Session == nil {
		return offset - 1
	}
	return offset
}

// Offset is the index of the first row Window drew, so a pointer row can be
// mapped back to a Row (#45).
func (s *Sidebar) Offset() int { return s.offset }

// MoveDown advances to the next session row, wrapping to the first (#126).
func (s *Sidebar) MoveDown() { s.seek(1) }

// MoveUp retreats to the previous session row, wrapping to the last (#126).
func (s *Sidebar) MoveUp() { s.seek(-1) }

// NextProject moves to the first session of the next project that has one,
// wrapping past the last project (#130).
func (s *Sidebar) NextProject() { s.jumpProject(1) }

// PrevProject moves to the first session of the previous project that has
// one - the first, not the last, so ] and [ are not each other's inverse in
// the naive way (#130).
func (s *Sidebar) PrevProject() { s.jumpProject(-1) }

// jumpProject walks header rows in step's direction, skipping the current
// project's own header (going up would otherwise stop at it) and any project
// with no session beneath it, and lands on the first session after the first
// header that qualifies. One lap at most, as in seek.
func (s *Sidebar) jumpProject(step int) {
	n, current := len(s.rows), s.currentProject()
	i := s.cursor
	for lap := 0; lap < n; lap++ {
		i = ((i+step)%n + n) % n
		if s.rows[i].Session != nil || s.rows[i].Project == current {
			continue
		}
		if i+1 < n && s.rows[i+1].Session != nil {
			s.cursor = i + 1
			return
		}
	}
}

func (s *Sidebar) currentProject() string {
	if row, ok := s.Selected(); ok {
		return row.Project
	}
	return ""
}

// seek moves the cursor by step until it lands on a session row, taking the
// index modulo the row count so the two ends join. At most one lap: a list
// with no session row (a project registered with nothing under it) leaves
// the cursor where it was instead of spinning (#126). From the -1 sentinel
// NewSidebar and SetRows start at, a downward seek scans from row 0.
func (s *Sidebar) seek(step int) {
	n := len(s.rows)
	i := s.cursor
	for lap := 0; lap < n; lap++ {
		i = ((i+step)%n + n) % n
		if s.rows[i].Session != nil {
			s.cursor = i
			return
		}
	}
}
