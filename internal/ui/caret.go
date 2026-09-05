// The cursor omatty draws for the focused session. Split from render.go
// because placing it is a question about the pane's geometry rather than
// about the frame's contents (#106).

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
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
	if term == nil || m.review.Focused || m.review.Note.Active || m.sessionExited() {
		return nil
	}
	c := term.Cursor()
	if !c.Visible || !m.inPaneGrid(c.X, c.Y) {
		return nil
	}
	x, y := PaneOrigin()
	cursor := tea.NewCursor(x+c.X, y+c.Y)
	applyShape(cursor, c)
	return cursor
}

// sessionExited reports whether the selected session's process has ended.
//
// A caret over a dead pane invites typing into a PTY nobody is reading. On
// exit claude restores DECTCEM and leaves the alt screen, so the emulator goes
// on reporting a visible cursor at whatever cell the primary screen held, and
// nothing panicked - so Guard.Panicked is false too. The registry's status is
// the only thing that knows, and the sidebar already draws it (#106).
func (m *Model) sessionExited() bool {
	return m.status[m.Focused()].Status == watcher.StatusExited
}

// applyShape copies the emulator's cursor style over, unless it is reporting
// the DEC default.
//
// bubbleterm answers CursorAppearance with {Block, Blink: true} whether or not
// the child ever sent DECSCUSR, and forwarding that made bubbletea write
// DECSCUSR 1 to the host - so an operator whose terminal is set to a steady
// bar watched it become a blinking block for the whole omatty run. A child
// that really does ask for a blinking block gets one anyway: that is what the
// host's own default already is (#106).
func applyShape(cursor *tea.Cursor, c termwrap.Caret) {
	if c.Shape == tea.CursorBlock && c.Blink {
		return
	}
	cursor.Shape, cursor.Blink = c.Shape, c.Blink
}
