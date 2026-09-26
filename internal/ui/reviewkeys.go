package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/paste"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// ReviewCursor is the index of the highlighted row.
func (m *Model) ReviewCursor() int { return m.review.DiffList.Cursor }

// PendingComments is how many notes the shown session has queued.
func (m *Model) PendingComments() int { return m.commentsFor(m.review.SessionID).PendingLen() }

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
	if m.commentKey(key) {
		return nil
	}
	switch key {
	case "r":
		return m.loadDiff(m.review.SessionID)
	case "t":
		return m.toggleScope()
	case "o":
		return m.openPreviewAtCursor()
	// Two spellings, because a terminal reporting the shift modifier gives
	// "shift+s" while a legacy one gives the bare "S" (issue #87).
	case "shift+s", "S":
		return m.submitReview()
	}
	return nil
}

// commentKey runs the three keys that write or remove a note, reporting whether
// key was one. A second table because one switch over every review key is past
// the statement limit, and these three share a subject.
func (m *Model) commentKey(key string) bool {
	switch key {
	case "c":
		m.openNote()
	// Two spellings, as S has: a terminal reporting the shift modifier gives
	// "shift+c" and a legacy one the bare "C" (issue #87).
	case "shift+c", "C":
		m.openFragment()
	case "d":
		m.deleteComment()
	default:
		return false
	}
	return true
}

func (m *Model) moveReviewCursor(delta int) {
	m.review.DiffList.move(delta, len(m.review.Entries), m.reviewRows())
}

// reviewRows is how many entry rows the column shows: the pane minus its
// title, minus the editor line while it is open.
func (m *Model) reviewRows() int {
	_, h := PaneSize(m.width, m.height, true)
	if m.review.Note.Active || m.review.Filter.Active {
		return h - 1
	}
	return h
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
	if m.review.DiffList.Cursor >= len(m.review.Entries) {
		return review.Entry{}, false
	}
	return m.review.Entries[m.review.DiffList.Cursor], true
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
		Anchor: m.anchorAt(e.Pos),
		Quote:  m.shownDiff().LineAt(e.Pos).Text,
	}
}

// anchorAt anchors a note on the shown row. In the turn scope it anchors on
// the same line of the session diff, which Compose locates against: the
// turn's own hunk header matches nothing there, and falling back to the
// first line that reads the same told claude the wrong line (#311).
func (m *Model) anchorAt(p review.Position) review.Anchor {
	if m.review.Scope == scopeTurn {
		return review.AnchorFor(m.review.Diff, m.review.TurnDiff, p)
	}
	return review.AnchorAt(m.review.Diff, p)
}

// openFragment starts a note about part of the line under the cursor: the
// fragment is collected first, then the note (#339).
//
// Typed rather than selected, and that is a limit worth stating: #360's drag
// selection reads the *terminal* pane's cells, not the diff column's, so there
// is no pointer gesture over a diff line to reuse. Typing or pasting the words
// needs no column cursor, and it is checked against the line before anything is
// queued, which a range of columns could not be.
func (m *Model) openFragment() {
	m.openNote()
	if m.review.Note.Active {
		m.review.Note.Stage = stageFragment
	}
}

// onNoteKey edits the note; enter commits the stage, esc discards the whole
// note. The keystroke handling is editKey, shared with the modal editors (#41).
func (m *Model) onNoteKey(msg tea.KeyPressMsg) tea.Cmd {
	buffer, action := editKey(m.review.Note.Buffer, msg)
	m.review.Note.Buffer = buffer
	switch action {
	case editCancel:
		m.review.Note = noteEditor{}
	case editCommit:
		m.commitNoteStage()
	}
	return nil
}

// commitNoteStage moves the fragment prompt on to the note, or queues a note
// that is ready.
func (m *Model) commitNoteStage() {
	if m.review.Note.Stage != stageFragment {
		m.queueNote()
		return
	}
	fragment := strings.TrimSpace(m.review.Note.Buffer)
	if fragment == "" {
		return // an empty prompt waits, the way queueNote's empty note does
	}
	if !strings.Contains(m.review.Note.Quote, fragment) {
		// Caught while the line is still on screen, rather than reaching claude
		// as a quotation of something that was never there.
		m.lastErr = strconv.Quote(fragment) + " is not on this line"
		m.review.Note = noteEditor{}
		return
	}
	m.review.Note.Fragment, m.review.Note.Buffer, m.review.Note.Stage = fragment, "", stageNote
}

// queueNote stores the note. An empty note leaves the editor open rather than
// queueing nothing.
func (m *Model) queueNote() {
	note := strings.TrimSpace(m.review.Note.Buffer)
	if note == "" {
		return
	}
	n := m.review.Note
	m.commentsFor(m.review.SessionID).Add(review.Comment{
		Anchor: n.Anchor, Quote: n.Quote, Note: note, Fragment: n.Fragment,
	})
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

// submitReview sends every pending comment as one message (#23, invariant 8)
// and hands focus back to the terminal so the operator watches claude act on
// it. What it sent stays on its line marked sent rather than vanishing, so the
// next turn can be read against what was asked (#335).
func (m *Model) submitReview() tea.Cmd {
	cs := m.commentsFor(m.review.SessionID)
	pending := cs.Pending()
	if len(pending) == 0 {
		m.lastErr = "no comments to submit; press c on a diff line first"
		return nil
	}
	term := m.terms[m.review.SessionID]
	if term == nil {
		m.lastErr = "session " + m.review.SessionID + " has no terminal to send to"
		return nil
	}
	body := review.Compose(m.review.Diff, pending)
	cs.MarkSent(m.clock())
	cs.PruneSent(m.review.Diff) // one sent while already moved has nothing left to mark
	m.review.Focused = false
	m.rebuildEntries()
	return term.SendInput(paste.BracketedPaste(body))
}
