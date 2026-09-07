package ui

import tea "charm.land/bubbletea/v2"

// A left click on a sidebar row selects it (#45). It is the same action j/k
// take - size the terminal it lands on (#73) and drag an open review column
// with it (#21) - so it goes through moveCursor rather than touching the
// cursor directly. The three ways to change the selection must not do three
// amounts of work.

// clickSidebar selects the session row under a left click. Everything else
// does nothing: the pane's own pointer belongs to claude, the review column
// has no row model, and a project header is a label rather than a target
// (sidebar.go). A modal owns the whole pane surface, so a click behind one is
// dropped too - the row under the pointer is not on screen.
func (m *Model) clickSidebar(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Button != tea.MouseLeft || m.modalOpen() {
		return nil
	}
	if m.overReview(msg.X) || !m.overSidebar(msg.X) {
		return nil
	}
	i, ok := m.sidebarRowAt(msg.Y)
	if !ok {
		return nil
	}
	return m.selectRow(i)
}

// sidebarRowAt maps a window row to an index into Sidebar.Rows, undoing what
// renderSidebar did: the top border, the pinned header, and the scroll offset
// the frame was drawn with. ok is false for a header row, an empty row, and
// anything outside the box (#45, #129).
func (m *Model) sidebarRowAt(winY int) (int, bool) {
	i := winY - sidebarTop() + m.sidebarOffset()
	rows := m.sidebar.Rows()
	if i < 0 || i >= len(rows) || rows[i].Session == nil {
		return 0, false
	}
	return i, true
}

// sidebarOffset recomputes the window from the cursor rather than trusting
// the last frame's, so a click before any View sees what View would draw.
func (m *Model) sidebarOffset() int {
	_, h := PaneSize(m.width, m.height, m.review.Open)
	m.sidebar.Window(h - sidebarHeaderRows)
	return m.sidebar.Offset()
}

// selectRow puts the cursor on a row index. A click on the row already
// selected returns nothing, so it cannot re-resize a terminal that did not
// change.
func (m *Model) selectRow(i int) tea.Cmd {
	id := m.sidebar.Rows()[i].Session.ID
	if m.Selected() == id {
		return nil
	}
	return m.moveCursor(func() { m.sidebar.SelectByID(id) })
}
