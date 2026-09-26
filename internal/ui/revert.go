// Putting a session's worktree back to where its last turn started (#334).
//
// One key, one confirmation, and the confirmation names how many files would
// go. Destructive by design: the whole point is that undoing a bad change
// should be cheaper than preventing it, and a safety net nobody trusts is not
// a safety net.

package ui

import (
	"errors"
	"log/slog"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// RevertedMsg carries the outcome of a revert into Update. Exported so tests
// can send one.
type RevertedMsg struct {
	SessionID string
	Files     int
	Err       error
}

// revertKey is the answer that actually discards the turn. Not enter, and not
// the key that opened the box: the same argument archiveChoices makes for `w`
// (#40), because the hand that pressed a key once will press it again.
const revertKey = "u"

// askRevert opens the confirmation, or refuses and says why.
//
// Four refusals, each with its own sentence. A key that appears to do nothing
// reads as a bug, and the four are genuinely different situations: the session
// is busy, no baseline was ever taken, hooks are down so no baseline ever will
// be, or the turn changed nothing worth discarding.
func (m *Model) askRevert() tea.Cmd {
	row, ok := m.sidebar.Selected()
	if !ok {
		return nil
	}
	if !atRest(m.reportedStatus(row.Session.ID)) || m.turnPending[row.Session.ID] {
		m.lastErr = "cannot revert " + row.Session.Title + " mid-turn; wait for it to finish"
		return nil
	}
	files, err := m.turn.Count(*row.Session)
	if err != nil {
		m.refuseRevert(row.Session.Title, err)
		return nil
	}
	if files == 0 {
		m.notice = row.Session.Title + " has changed nothing since its turn began"
		return nil
	}
	m.openRevertBox(row, files)
	return nil
}

// refuseRevert names why there is no baseline to go back to, in #311's own
// vocabulary rather than a raw error the operator cannot act on.
func (m *Model) refuseRevert(title string, err error) {
	if errors.Is(err, review.ErrNoTurn) {
		m.notice = "no turn recorded for " + title + " yet; there is nothing to go back to"
		return
	}
	slog.Warn("counting a turn's changes", "session", title, "err", err)
	m.lastErr = err.Error()
}

// openRevertBox asks the question, with the count in it.
func (m *Model) openRevertBox(row Row, files int) {
	m.openModal(modal{Kind: modalRevert, Confirm: confirmBox{
		SessionID: row.Session.ID,
		Title:     row.Session.Title,
		Question:  "revert " + strconv.Quote(row.Session.Title) + " to the start of its last turn?",
		Note:      revertNote(files),
		Choices: []confirmChoice{{
			Key:   revertKey,
			Label: "revert it",
			Warn:  "discards everything this turn changed",
		}},
	}})
}

// revertNote is what the operator is agreeing to lose, counted before anything
// is written so the number they saw is the number that goes.
func revertNote(files int) string {
	if files == 1 {
		return "1 file will be discarded"
	}
	return strconv.Itoa(files) + " files will be discarded"
}

// revertSession runs the restore off the Update goroutine: it rewrites a
// working tree, which is slower than a frame on a large one.
func (m *Model) revertSession() tea.Cmd {
	id := m.modal.Confirm.SessionID
	m.modal = modal{}
	sess, ok := m.session(id)
	if !ok {
		return nil
	}
	revert := m.turn.Revert
	return func() tea.Msg {
		files, err := revert(sess)
		return RevertedMsg{SessionID: id, Files: files, Err: err}
	}
}

// onReverted reports what happened. A worktree left half restored in silence is
// the one failure the operator cannot see from inside the terminal.
//
// Telling the session is deliberately not done here: sending anything is S's
// job and needs a person (#334). claude will read the files when it next looks.
func (m *Model) onReverted(msg RevertedMsg) tea.Cmd {
	if msg.Err != nil {
		slog.Warn("reverting a turn", "session", msg.SessionID, "err", msg.Err)
		m.lastErr = msg.Err.Error()
		return nil
	}
	m.notice = "reverted " + strconv.Itoa(msg.Files) + " file(s) to the start of the last turn"
	return m.refreshAfterRevert(msg.SessionID)
}

// refreshAfterRevert re-reads what the column and the card show: the diff, the
// listing and the diffstat are all about a working tree that just changed under
// them.
func (m *Model) refreshAfterRevert(id string) tea.Cmd {
	return tea.Batch(m.loadDiff(id), m.relistFiles(id), m.pollStat(id))
}

// reportedStatus is a session's status with an unreported one read as idle,
// which is what sessionRows already does for the sidebar (#334).
//
// Status is a string, so its zero value is "" and not StatusIdle - and atRest
// answers false for it. A session nothing has reported on is not mid-turn: it is
// a session that has not spoken yet, which is every session at boot and every
// stopped one. Without this, u refused to revert with "cannot revert mid-turn"
// on exactly the sessions a revert is most useful for. Found by the real-PTY
// smoke test, not by a unit test - the fixtures all report a status first.
func (m *Model) reportedStatus(id string) watcher.Status {
	if s := m.status[id].Status; s != "" {
		return s
	}
	return watcher.StatusIdle
}
