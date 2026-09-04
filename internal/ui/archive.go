// Archiving a session (#40): the confirmation, the teardown, and the two
// dependencies that persist it. Until now the only way to be rid of a session
// was to edit state.json by hand.
//
// Archiving forgets a session; it does not destroy its history. The transcript
// stays on disk, so `claude --resume` still finds it - only omatty's memory of
// the session goes. The one genuinely destructive part is removing the
// worktree, which is why that has its own keystroke.

package ui

import (
	"fmt"
	"log/slog"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// ArchiveFunc drops a session from the registry. Injected so ui never reaches
// the store itself; cmd/omatty closes it over the store.
//
//	deps.Archive = func(id string) error {
//	        _, err := registry.RemoveSession(store, id)
//	        return err
//	}
type ArchiveFunc func(sessionID string) error

// RemoveWorktreeFunc deletes a linked worktree. Injected because ui may never
// shell out to git (invariant 4); the arguments mirror vcs.Git.RemoveWorktree.
type RemoveWorktreeFunc func(repoRoot, dir string) error

// noArchive is the Deps.Archive default. It names the missing wiring rather
// than appearing to succeed, which would drop a row from the sidebar that
// state.json still holds - the same reasoning as noDiff and noRename.
func noArchive(sessionID string) error {
	return fmt.Errorf("ui: no archive source configured for session %s", sessionID)
}

// noRemoveWorktree is the Deps.RemoveWorktree default, for the same reason. A
// silent no-op here would report a worktree deleted while it sat on disk.
func noRemoveWorktree(repoRoot, dir string) error {
	return fmt.Errorf("ui: no worktree remover configured for %q in %q", dir, repoRoot)
}

// noTailStop is the Deps.TailStop default: with no watcher running there is no
// tailer to stop, so doing nothing is correct rather than an error.
func noTailStop(string) {}

// confirmChoice is one answer in a confirmation. Key is the keystroke that
// picks it; esc is always cancel and is not listed here.
type confirmChoice struct {
	Key   string
	Label string
	// Warn is drawn under the label, indented. It is a separate line rather
	// than a longer label because the pane is 50 columns on an 80-column
	// window, and a warning that truncates is not a warning.
	Warn           string
	RemoveWorktree bool
}

// confirmBox is the pending archive confirmation. The zero value is closed.
type confirmBox struct {
	SessionID string
	Title     string
	Choices   []confirmChoice
}

// archiveChoices is the answer set for a session.
//
// A worktree session gets a second answer because `git worktree remove
// --force` discards uncommitted work silently: destroying it needs its own
// keystroke, never the one the hand reaches for. A main-checkout session gets
// no such option at all - omatty did not create that directory, so it must
// never delete it.
func archiveChoices(worktree bool) []confirmChoice {
	if !worktree {
		return []confirmChoice{{Key: "y", Label: "archive"}}
	}
	return []confirmChoice{
		{Key: "y", Label: "archive, keep the worktree"},
		{
			Key:            "w",
			Label:          "archive + remove worktree",
			Warn:           "discards uncommitted work",
			RemoveWorktree: true,
		},
	}
}

// openConfirm asks before archiving the selected session.
func (m *Model) openConfirm() {
	row, ok := m.sidebar.Selected()
	if !ok {
		return
	}
	m.modal = modal{Kind: modalConfirm, Confirm: confirmBox{
		SessionID: row.Session.ID,
		Title:     row.Session.Title,
		Choices:   archiveChoices(row.Session.Worktree),
	}}
}

// onConfirmKey answers the confirmation. Anything that is not an offered key
// is ignored rather than treated as a cancel, so a stray keystroke cannot
// dismiss the question the operator meant to answer.
func (m *Model) onConfirmKey(key string) tea.Cmd {
	if key == "esc" {
		m.modal = modal{}
		return nil
	}
	for _, c := range m.modal.Confirm.Choices {
		if c.Key == key {
			return m.archiveSession(c.RemoveWorktree)
		}
	}
	return nil
}

// archiveSession drops the session from the registry, then unwinds it.
//
// The registry edit comes first because it is the fallible step: a failed save
// must leave the session running and visible rather than half gone. That is
// restartFocused's discipline (#15) - do the thing that can fail, and destroy
// only after it succeeds.
func (m *Model) archiveSession(removeWorktree bool) tea.Cmd {
	id := m.modal.Confirm.SessionID
	m.modal, m.lastErr = modal{}, ""
	sess, ok := m.session(id)
	if !ok {
		return nil
	}
	if err := m.archive(id); err != nil {
		slog.Error("archiving session", "session", id, "err", err)
		m.lastErr = err.Error()
		return nil
	}
	return m.dropSession(sess, removeWorktree)
}

// dropSession unwinds everything the session owned: its process, its tailer,
// its per-session state and its sidebar row. Missing any one of these leaks;
// the tailer most visibly, since it would go on polling a transcript path
// whose directory is about to be deleted (#40).
func (m *Model) dropSession(sess registry.Session, removeWorktree bool) tea.Cmd {
	if term := m.terms[sess.ID]; term != nil {
		_ = term.Close()
	}
	// The map is the one ui.Run's deferred closeTerminals holds, so deleting
	// here is also what stops it being closed twice at exit (#72).
	delete(m.terms, sess.ID)
	m.tailStop(sess.ID)
	m.forgetSession(sess.ID)
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	// SetRows falls back to the first session row when the selected id is gone,
	// not to the neighbour, so the cursor can land anywhere - including in
	// another project. Size whatever it landed on and drag the review column
	// along: the pair moveCursor uses (#73, #95).
	cmds := []tea.Cmd{m.resizeSelected(), m.followSession()}
	if removeWorktree && sess.Worktree {
		cmds = append(cmds, m.removeWorktreeCmd(sess))
	}
	return tea.Batch(cmds...)
}

// forgetSession drops the session from the in-memory state and from every
// per-session map, and closes the review column if it was showing that
// session: its worktree may be about to disappear underneath it.
//
// Late hook events for the session need no guard of their own - onStatus
// already ignores an id the state does not hold (issue #69).
func (m *Model) forgetSession(id string) {
	for i := range m.state.Sessions {
		if m.state.Sessions[i].ID == id {
			m.state.Sessions = append(m.state.Sessions[:i], m.state.Sessions[i+1:]...)
			break
		}
	}
	delete(m.status, id)
	delete(m.notified, id)
	delete(m.comments, id)
	if m.review.SessionID == id {
		m.review = ReviewPane{}
	}
}

// WorktreeRemovedMsg carries the outcome of a worktree removal into Update.
// Exported so tests can send one.
type WorktreeRemovedMsg struct {
	SessionID string
	Dir       string
	Err       error
}

// removeWorktreeCmd deletes the worktree off the Update goroutine: git on a
// large tree takes long enough to stall the frame.
func (m *Model) removeWorktreeCmd(sess registry.Session) tea.Cmd {
	root, remove := m.projectRoot(sess.Project), m.removeWorktree
	return func() tea.Msg {
		return WorktreeRemovedMsg{SessionID: sess.ID, Dir: sess.Dir, Err: remove(root, sess.Dir)}
	}
}

// onWorktreeRemoved surfaces a failed removal. The session is already out of
// the registry by the time this arrives and re-adding it would be worse than
// the leak, so the operator is told which directory is still on disk instead.
func (m *Model) onWorktreeRemoved(msg WorktreeRemovedMsg) tea.Cmd {
	if msg.Err == nil {
		return nil
	}
	slog.Error("removing worktree", "session", msg.SessionID, "dir", msg.Dir, "err", msg.Err)
	m.lastErr = "worktree " + strconv.Quote(msg.Dir) + " left on disk: " + msg.Err.Error()
	return nil
}
