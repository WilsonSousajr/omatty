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

// Sidebar holds the row list and the cursor. The cursor rests on a session
// row, or on the header of a project with no session beneath it: that header
// is the only row that can name such a project, and every project-scoped
// action reads the cursor (#158). A header with sessions is still a label -
// its first session stands for it.
type Sidebar struct {
	rows   []Row
	cursor int
	offset int // first row drawn; recomputed by Window each frame (#129)
}

// NewSidebar returns a Sidebar with the cursor on the first session row, or
// on the first project header when no project has one.
func NewSidebar(rows []Row) *Sidebar {
	s := &Sidebar{rows: rows}
	s.reset()
	return s
}

// reset puts the cursor on the first session anywhere, and only with no
// session registered at all on the first landing row. A fresh omatty opens on
// something to type into when there is one, not on an empty project that
// happens to be registered first (#158).
func (s *Sidebar) reset() {
	s.cursor = -1
	for i, r := range s.rows {
		if r.Session != nil {
			s.cursor = i
			return
		}
	}
	s.MoveDown()
}

// Rows returns the rows in display order, for rendering.
func (s *Sidebar) Rows() []Row { return s.rows }

// SetRows replaces the rows while keeping the cursor on the same session if it
// is still present, so a live status update does not move the selection. A
// session that is gone - archived - keeps the cursor in its project: on the
// project's next session, or on its header once it holds none, so ctrl+o n
// recreates there rather than wherever reset lands (#158).
func (s *Sidebar) SetRows(rows []Row) {
	var selectedID string
	if sel, ok := s.Selected(); ok {
		selectedID = sel.Session.ID
	}
	project := s.CursorProject()
	s.rows = rows
	s.reset()
	if !s.SelectByID(selectedID) {
		s.SelectByProject(project)
	}
}

// SelectByProject puts the cursor on a project's first session, or on its
// header when it has none, and reports whether the project is registered.
//
//	if sb.SelectByProject("wstech") { /* ctrl+o n now creates in wstech */ }
func (s *Sidebar) SelectByProject(name string) bool {
	for i, r := range s.rows {
		if r.Project == name && s.landable(i) {
			s.cursor = i
			return true
		}
	}
	return false
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
// sidebar holds no sessions, and when the cursor rests on an empty project's
// header - so every caller that dereferences row.Session is already guarded
// against the one row that has none (#158).
func (s *Sidebar) Selected() (Row, bool) {
	if !s.onRow() || s.rows[s.cursor].Session == nil {
		return Row{}, false
	}
	return s.rows[s.cursor], true
}

// SelectedHeader is the project whose header the cursor rests on, which only
// happens for a project with no sessions (#158). ok is false when the cursor
// is on a session or on nothing.
//
//	if p, ok := sb.SelectedHeader(); ok { /* ctrl+o x may forget p */ }
func (s *Sidebar) SelectedHeader() (string, bool) {
	if !s.onRow() || s.rows[s.cursor].Session != nil {
		return "", false
	}
	return s.rows[s.cursor].Project, true
}

// CursorProject is the project the cursor is in, whether it rests on a
// session or on an empty header; "" with nothing to rest on. This is what
// ctrl+o n, N and A read (#158).
//
//	project := sb.CursorProject()
func (s *Sidebar) CursorProject() string {
	if !s.onRow() {
		return ""
	}
	return s.rows[s.cursor].Project
}

func (s *Sidebar) onRow() bool { return s.cursor >= 0 && s.cursor < len(s.rows) }

// landable reports whether the cursor may rest on row i: a session, or the
// header of a project with nothing beneath it (#158).
func (s *Sidebar) landable(i int) bool {
	return s.rows[i].Session != nil || s.emptyHeader(i)
}

// emptyHeader reports whether row i is a header with no session under it.
func (s *Sidebar) emptyHeader(i int) bool {
	if s.rows[i].Session != nil {
		return false
	}
	return i+1 == len(s.rows) || s.rows[i+1].Session == nil
}

// selectIndex puts the cursor on row i if the cursor may rest there. The
// click path's entry (#45, #158).
func (s *Sidebar) selectIndex(i int) bool {
	if i < 0 || i >= len(s.rows) || !s.landable(i) {
		return false
	}
	s.cursor = i
	return true
}

// cardLines is how many lines a session draws: the glyph, title and age, then
// the branch, diffstat and lane (#176). A header draws one.
const cardLines = 2

// rowHeight is the lines a row draws. The window math and the click inverse
// both read it, so the two cannot drift (#45, #129, #176).
func rowHeight(r Row) int {
	if r.Session == nil {
		return 1
	}
	return cardLines
}

// Window returns the rows to draw when only lines fit - in lines, a card
// being two and a header one (#176) - keeping the cursor's whole card inside
// them. The offset is recomputed from the cursor on every call rather than
// trusted from the last move, because a resize can shrink the pane after the
// cursor last moved - the same reason renderTree does it (#129).
//
//	for _, row := range sb.Window(paneRows) { ... }
func (s *Sidebar) Window(lines int) []Row {
	s.offset = s.revealHeader(s.scrollTo(lines))
	end := s.offset
	for budget := lines; end < len(s.rows) && rowHeight(s.rows[end]) <= budget; end++ {
		budget -= rowHeight(s.rows[end])
	}
	if s.offset >= end {
		return nil
	}
	return s.rows[s.offset:end]
}

// scrollTo is the first row to draw so the cursor's whole card fits in lines,
// moving the offset as little as possible: back to the cursor when it is
// above the window, forward one row at a time until the rows from the offset
// to the cursor fit when it is below. ScrollOffset in lines rather than rows,
// which is why it is not ScrollOffset.
func (s *Sidebar) scrollTo(lines int) int {
	c := max(s.cursor, 0)
	if c >= len(s.rows) {
		return 0
	}
	off := min(s.offset, c)
	for off < c && s.linesBetween(off, c+1) > lines {
		off++
	}
	return off
}

// linesBetween is how many lines rows [from, to) draw.
func (s *Sidebar) linesBetween(from, to int) int {
	n := 0
	for _, r := range s.rows[from:to] {
		n += rowHeight(r)
	}
	return n
}

// rowAtLine is the row drawn line lines below the first drawn one, walking
// the heights Window drew with, so a click on either line of a card lands on
// it (#45, #176). ok is false above the list and past its end.
func (s *Sidebar) rowAtLine(line int) (int, bool) {
	if line < 0 {
		return 0, false
	}
	for i := s.offset; i < len(s.rows); i++ {
		if line < rowHeight(s.rows[i]) {
			return i, true
		}
		line -= rowHeight(s.rows[i])
	}
	return 0, false
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

// NextProject moves to the next project: its first session, or its header
// when it has none, wrapping past the last project (#130, #158).
func (s *Sidebar) NextProject() { s.jumpProject(1) }

// PrevProject moves to the previous project - its first session, not its
// last, so ] and [ are not each other's inverse in the naive way - or to its
// header when it has none (#130, #158).
func (s *Sidebar) PrevProject() { s.jumpProject(-1) }

// jumpProject walks header rows in step's direction, skipping the current
// project's own header (going up would otherwise stop at it), and lands on
// the first session after the first other header - or on that header itself
// when nothing is beneath it (#130, #158). One lap at most, as in seek.
func (s *Sidebar) jumpProject(step int) {
	n, current := len(s.rows), s.CursorProject()
	i := s.cursor
	for lap := 0; lap < n; lap++ {
		i = ((i+step)%n + n) % n
		if s.rows[i].Session != nil || s.rows[i].Project == current {
			continue
		}
		if s.emptyHeader(i) {
			s.cursor = i
			return
		}
		s.cursor = i + 1
		return
	}
}

// seek moves the cursor by step until it lands on a row the cursor may rest
// on, taking the index modulo the row count so the two ends join. At most one
// lap: an empty list leaves the cursor where it was instead of spinning
// (#126). From the -1 sentinel reset starts at, a downward seek scans from
// row 0.
func (s *Sidebar) seek(step int) {
	n := len(s.rows)
	i := s.cursor
	for lap := 0; lap < n; lap++ {
		i = ((i+step)%n + n) % n
		if s.landable(i) {
			s.cursor = i
			return
		}
	}
}
