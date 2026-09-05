// The cursor omatty draws for the focused session. Split from render.go
// because placing it is a question about the pane's geometry rather than
// about the frame's contents (#106).

package ui

import (
	tea "charm.land/bubbletea/v2"
)

// paneCursor is where bubbletea should put the terminal cursor: the emulated
// cursor of the focused session, offset by the pane's origin.
//
// Nil means "leave it hidden", which is what every frame did before this and
// what bubbletea does with a nil View.Cursor. Claude draws no caret of its
// own - it moves the real one - so without this the prompt gives no sign of
// where you are typing (issue #106).
func (m *Model) paneCursor() *tea.Cursor {
	term := m.focusedTerminal()
	if term == nil || m.review.Focused {
		return nil
	}
	c := term.Cursor()
	if !c.Visible || !m.inPaneGrid(c.X, c.Y) {
		return nil
	}
	x, y := PaneOrigin()
	cursor := tea.NewCursor(x+c.X, y+c.Y)
	cursor.Shape, cursor.Blink = c.Shape, c.Blink
	return cursor
}
