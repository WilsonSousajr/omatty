// A left click on the review column (#168). Since the sidebar got a mouse
// (#45) the column had been the one list on screen the pointer could not
// touch: clickSidebar dropped it because the column had no row model, and no
// pane drew a close affordance at all. The operator reached for the × that
// should have been there, clicked, and got silence - indistinguishable from
// a broken button.
//
// Clicks inside the session pane stay claude's: that pointer belongs to the
// program in the PTY (#107).

package ui

import tea "charm.land/bubbletea/v2"

// click routes a left click to the column under it. The review column is
// tried first because overReview includes its hairline; the sidebar keeps
// its own gate.
func (m *Model) click(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Button == tea.MouseLeft && !m.modalOpen() && m.review.Open && m.overReview(msg.X) {
		return m.clickReview(msg)
	}
	return m.clickSidebar(msg)
}

// clickReview closes the column from the × on its rule, or moves the cursor
// to the row under the pointer and gives the column the keys, the same
// action j/k take. A modal owns the whole surface, so a click behind one is
// dropped by click before this is reached, as it is for the sidebar.
func (m *Model) clickReview(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Y == ruleY() && msg.X == m.reviewCloseX() {
		return m.closeColumn()
	}
	if delta, ok := m.reviewRowDelta(msg.Y); ok {
		m.review.Focused = true
		m.moveReviewRow(delta)
	}
	return nil
}

// ruleY is the window row of the rule, and reviewTop the first content row
// of the column: the header row and the rule above it (#174).
func ruleY() int     { return headerRows }
func reviewTop() int { return headerRows + ruleRows }

// reviewCloseX is the window column of the ×: the column is drawn flush to
// the window's right edge and the glyph is its rule's last cell.
func (m *Model) reviewCloseX() int { return m.width - 1 }

// reviewRowDelta maps a window row to how far the current view's cursor must
// move to land on the row drawn there, undoing what the renderer did: the
// chrome above, then the scroll offset the frame was drawn with, recomputed
// rather than trusted the way sidebarRowAt does (#45, #129). The preview
// has no cursor.
func (m *Model) reviewRowDelta(winY int) (int, bool) {
	line := winY - reviewTop()
	rows := m.reviewRows()
	if line < 0 || line >= rows {
		return 0, false
	}
	cursor, offset, count := m.reviewCursorState()
	if count == 0 {
		return 0, false
	}
	target := ScrollOffset(cursor, offset, rows) + line
	if target >= count {
		return 0, false
	}
	return target - cursor, true
}

// reviewCursorState is the cursor, offset and row count of the view that has
// one; a preview reports no rows.
func (m *Model) reviewCursorState() (cursor, offset, count int) {
	switch m.review.View {
	case ViewDiff:
		return m.review.Cursor, m.review.Offset, len(m.review.Entries)
	case ViewTree:
		return m.review.TreeCursor, m.review.TreeOffset, len(m.treeRows())
	}
	return 0, 0, 0
}

// moveReviewRow moves the current view's cursor by delta through the same
// clamps j and k use.
func (m *Model) moveReviewRow(delta int) {
	if m.review.View == ViewTree {
		m.moveTreeCursor(delta)
		return
	}
	m.moveReviewCursor(delta)
}
