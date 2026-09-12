// a attaches the selected file to the session as an @path reference (#199):
// the tree as a way of pointing claude at code, which is what makes it an
// ADE surface rather than a file browser.

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/paste"
)

// attachSelected attaches the row under the tree cursor.
func (m *Model) attachSelected() tea.Cmd {
	rows := m.treeRows()
	if m.review.TreeCursor >= len(rows) {
		return nil
	}
	n := rows[m.review.TreeCursor]
	return m.attachPath(n.Path, n.IsDir)
}

// attachPath writes "@path " to the session's terminal inside paste brackets
// with no carriage return (invariant 8: the operator keeps typing, and a CR
// would submit a prompt of one path), then hands focus back the way
// submitReview does, so the next keystroke lands in claude's composer. A
// directory goes as "@dir/" so claude reads it as one. Without a terminal
// the footer says so and the column keeps the keys.
func (m *Model) attachPath(rel string, isDir bool) tea.Cmd {
	term := m.terms[m.review.SessionID]
	if term == nil {
		m.lastErr = "session " + m.review.SessionID + " has no terminal to attach to"
		return nil
	}
	if isDir {
		rel += "/"
	}
	m.review.Focused = false
	return term.SendInput(paste.BracketedText("@" + rel + " "))
}
