// A paste from the host terminal (#190).
//
// Bracketed paste is on, so the host wraps a paste in ESC[200~ ... ESC[201~
// and bubbletea hands the model a PasteMsg rather than keystrokes. Nothing
// routed it: it fell through Update's default to the broadcast every
// terminal ignores, and no PTY ever saw it - the one input path that failed
// without a trace. Invariant 1 covers keystrokes; a paste is not one, so it
// gets the same routing table by hand.

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// onPaste delivers a paste to whatever owns the keyboard, the way dispatch
// does for a key: the note editor or the filter line take it as text, the
// focused terminal takes it re-bracketed and without a carriage return (the
// operator pasted, not pressed enter - invariant 8), and the review column
// and a modal, which own no text field, drop it rather than type it into
// claude behind them. Never broadcast: a paste is for one pane (#190).
func (m *Model) onPaste(msg tea.PasteMsg) tea.Cmd {
	target, focused := m.focus()
	if !focused {
		return nil
	}
	switch target {
	case focusNote:
		m.review.Note.Buffer = editPaste(m.review.Note.Buffer, msg.Content)
	case focusFilter:
		m.setTreeFilter(editPaste(m.review.Filter.Query, msg.Content))
	case focusTerminal:
		return m.focusedTerminal().SendInput(review.BracketedText(msg.Content))
	}
	return nil
}
