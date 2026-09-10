package ui

import tea "charm.land/bubbletea/v2"

// A left click on a sidebar row selects it (#45). It is the same action j/k
// take - size the terminal it lands on (#73) and drag an open review column
// with it (#21) - so it goes through moveCursor rather than touching the
// cursor directly. The three ways to change the selection must not do three
// amounts of work.

// clickSidebar selects the row under a left click: a session, or the header
// of a project with none (#158). Everything else does nothing: the pane's own
// pointer belongs to claude, the review column has no row model, and a header
// with sessions is a label rather than a target (sidebar.go). A modal owns the whole pane surface, so a click behind one is
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
// View did: the header row and the rule, then the drawn rows by their
// heights, so either line of a card is the card (#45, #129, #176). ok is
// false for a header with sessions under it, an empty line, and anything
// outside the sidebar (#158).
func (m *Model) sidebarRowAt(winY int) (int, bool) {
	m.syncSidebarWindow()
	i, ok := m.sidebar.rowAtLine(winY - sidebarTop())
	if !ok || !m.sidebar.landable(i) {
		return 0, false
	}
	return i, true
}

// syncSidebarWindow recomputes the window from the cursor rather than
// trusting the last frame's, so a click before any View sees what View would
// draw.
func (m *Model) syncSidebarWindow() {
	_, h := PaneSize(m.width, m.height, m.review.Open)
	m.sidebar.Window(h - sidebarHeaderRows)
}

// selectRow puts the cursor on a row index. A click on the row already
// selected returns nothing, so it cannot re-resize a terminal that did not
// change.
func (m *Model) selectRow(i int) tea.Cmd {
	if i == m.sidebar.cursor {
		return nil
	}
	return m.moveCursor(func() { m.sidebar.selectIndex(i) })
}
