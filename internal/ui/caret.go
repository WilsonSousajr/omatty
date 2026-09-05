// The cursor omatty draws for the focused session. Split from render.go
// because placing it is a question about the pane's geometry rather than
// about the frame's contents (#106).

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
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
	if !c.Visible || !m.caretInPane(c) {
		return nil
	}
	x, y := PaneOrigin()
	cursor := tea.NewCursor(x+c.X, y+c.Y)
	cursor.Shape, cursor.Blink = c.Shape, c.Blink
	return cursor
}

// caretInPane reports whether the emulated cursor is on a cell the pane
// actually draws. fitBlock cuts everything past the pane, so a caret outside
// it would sit against content that is not on screen. The pane's last row is
// the title's, hence PTYSize rather than PaneSize.
func (m *Model) caretInPane(c termwrap.Caret) bool {
	w, h := PTYSize(m.width, m.height, m.review.Open)
	return c.X >= 0 && c.X < w && c.Y >= 0 && c.Y < h
}
