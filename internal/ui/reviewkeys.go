package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// ReviewCursor is the index of the highlighted row.
func (m *Model) ReviewCursor() int { return m.review.Cursor }

// PendingComments is how many notes the shown session has queued.
func (m *Model) PendingComments() int { return m.commentsFor(m.review.SessionID).Len() }

// onReviewKey handles a plain keystroke while the review column has focus.
// esc and ctrl+c hand focus back to the terminal but keep the column open;
// ctrl+c does not quit here, because it is the reflex for interrupting claude
// and a reviewer's hand is still on it (issue #28).
func (m *Model) onReviewKey(key string) tea.Cmd {
	switch key {
	case "j", "down":
		m.moveReviewCursor(1)
	case "k", "up":
		m.moveReviewCursor(-1)
	case "esc", "ctrl+c":
		m.review.Focused = false
	default:
		if m.panKey(key) {
			return nil
		}
		return m.reviewAction(key)
	}
	return nil
}

// reviewAction runs the commands that act on the row under the cursor.
func (m *Model) reviewAction(key string) tea.Cmd {
	switch key {
	case "c":
		m.openNote()
	case "d":
		m.deleteComment()
	case "r":
		return m.loadDiff(m.review.SessionID)
	// Two spellings, because a terminal reporting the shift modifier gives
	// "shift+s" while a legacy one gives the bare "S" (issue #87).
	case "shift+s", "S":
		return m.submitReview()
	}
	return nil
}

func (m *Model) moveReviewCursor(delta int) {
	n := len(m.review.Entries)
	if n == 0 {
		return
	}
	m.review.Cursor = min(max(m.review.Cursor+delta, 0), n-1)
	m.review.Offset = ScrollOffset(m.review.Cursor, m.review.Offset, m.reviewRows())
}

// reviewRows is how many entry rows the column shows: the pane minus its
// title, minus the editor line while it is open.
func (m *Model) reviewRows() int {
	_, h := PaneSize(m.width, m.height, true)
	if m.review.Note.Active {
		return h - 2
	}
	return h - 1
}

// ScrollOffset keeps cursor within the rows-high window that starts at offset,
// moving the window as little as possible.
//
//	off := ui.ScrollOffset(cursor, off, rows)
func ScrollOffset(cursor, offset, rows int) int {
	if rows <= 0 {
		return 0
	}
	if cursor < offset {
		return cursor
	}
	if cursor >= offset+rows {
		return cursor - rows + 1
	}
	return offset
}

func (m *Model) cursorEntry() (review.Entry, bool) {
	if m.review.Cursor >= len(m.review.Entries) {
		return review.Entry{}, false
	}
	return m.review.Entries[m.review.Cursor], true
}

// openNote starts a note on the line under the cursor, capturing its anchor
// and text now (invariant 7). Headers and existing comments are not lines.
func (m *Model) openNote() {
	e, ok := m.cursorEntry()
	if !ok || e.Kind != review.EntryLine {
		return
	}
	m.review.Note = noteEditor{
		Active: true,
		Anchor: review.AnchorAt(m.review.Diff, e.Pos),
		Quote:  m.review.Diff.LineAt(e.Pos).Text,
	}
}

// onNoteKey edits the note; enter queues it, esc discards it. The keystroke
// handling is editKey, shared with the modal editors (#41).
func (m *Model) onNoteKey(msg tea.KeyPressMsg) tea.Cmd {
	buffer, action := editKey(m.review.Note.Buffer, msg)
	m.review.Note.Buffer = buffer
	switch action {
	case editCancel:
		m.review.Note = noteEditor{}
	case editCommit:
		m.queueNote()
	}
	return nil
}

// queueNote stores the note. An empty note leaves the editor open rather than
// queueing nothing.
func (m *Model) queueNote() {
	note := strings.TrimSpace(m.review.Note.Buffer)
	if note == "" {
		return
	}
	n := m.review.Note
	m.commentsFor(m.review.SessionID).Add(review.Comment{Anchor: n.Anchor, Quote: n.Quote, Note: note})
	m.review.Note = noteEditor{}
	m.rebuildEntries()
}

// deleteComment removes the comment under the cursor, whether placed or
// orphaned. On any other row it does nothing, so d on a diff line cannot
// silently drop a note.
func (m *Model) deleteComment() {
	e, ok := m.cursorEntry()
	if !ok || (e.Kind != review.EntryComment && e.Kind != review.EntryOrphan) {
		return
	}
	m.commentsFor(m.review.SessionID).Remove(e.Comment)
	m.rebuildEntries()
}

// submitReview sends every queued comment as one message (#23, invariant 8)
// and hands focus back to the terminal so the operator watches claude act on
// it. The queue is cleared first: a comment that was sent is not pending.
func (m *Model) submitReview() tea.Cmd {
	cs := m.commentsFor(m.review.SessionID)
	if cs.Len() == 0 {
		m.lastErr = "no comments to submit; press c on a diff line first"
		return nil
	}
	term := m.terms[m.review.SessionID]
	if term == nil {
		m.lastErr = "session " + m.review.SessionID + " has no terminal to send to"
		return nil
	}
	body := review.Compose(m.review.Diff, cs.All())
	cs.Clear()
	m.review.Focused = false
	m.rebuildEntries()
	return term.SendInput(review.BracketedPaste(body))
}
