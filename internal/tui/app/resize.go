package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// onResize gives the terminal pane whatever the sidebar and diff pane leave.
// Off a tty bubbletea reports 0x0, which would floor every pane; the default
// stands instead (issue #74).
func (m *Model) onResize(msg tea.WindowSizeMsg) tea.Cmd {
	if msg.Width == 0 || msg.Height == 0 {
		return nil
	}
	m.width, m.height = msg.Width, msg.Height
	// A wider window raises the review column's width, which lowers the
	// ceiling on how far it may be panned. Nothing else re-clamps ColOffset, so
	// an offset left over from a narrow window made panLine drop every row
	// shorter than it and the column rendered blank until h/l was pressed - the
	// same "a resize reaches nothing" class #95 is about. renderEntries and
	// renderTree already recompute their vertical offsets for this reason.
	m.panReview(0)
	return m.resizeSelected()
}

// moveCursor moves the sidebar cursor, sizes the terminal it lands on (issue
// #73) and moves an open review column along with it (#21). It sizes what the
// cursor selects, not what holds the keyboard - the distinction #95 drew.
func (m *Model) moveCursor(move func()) tea.Cmd {
	move()
	return tea.Batch(m.resizeSelected(), m.followSession())
}

// ptySize is the live embedded-terminal size for the current window.
func (m *Model) ptySize() (int, int) { return PTYSize(m.width, m.height, m.review.Open) }

// resizeSelected sizes the selected session's terminal to the pane, whether or
// not a prompt currently owns the keyboard (issue #95). Only the selected
// terminal follows the window (issue #34), so the one just selected may still
// be at the size it was born or last selected at (issue #73).
func (m *Model) resizeSelected() tea.Cmd {
	term := m.selectedTerminal()
	if term == nil {
		return nil
	}
	return term.Resize(m.ptySize())
}

// selectedTerminal is the terminal the sidebar cursor is on, whether or not any
// surface currently owns the keyboard. Layout asks this one; key routing asks
// focusedTerminal. Answering both questions with one nil is issue #95: a resize
// arriving behind an open prompt reached no terminal at all.
//
// focusedTerminal is not "does the PTY own the keyboard" either - it only nils
// out for a modal, and the review column and note editor take keys without one.
// The question it answers is "no modal is open and a session is selected"; for
// the keyboard itself, ask focus().
//
// No guard on an empty id: a missing key yields the nil interface the caller
// already tests for, and a guard that cannot fire reads as an invariant.
func (m *Model) selectedTerminal() terminal.Terminal { return m.terms[m.Selected()] }

// focusedTerminal returns nil while a modal surface is open, which is what
// keeps its keys out of the PTY without special-casing the router: an
// unfocused terminal already routes every key to omatty. Only key routing and
// rendering may ask this - sizing the pane must not (issue #95).
func (m *Model) focusedTerminal() terminal.Terminal {
	if m.modalOpen() {
		return nil
	}
	return m.selectedTerminal()
}
