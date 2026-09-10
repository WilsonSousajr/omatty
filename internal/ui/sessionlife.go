// The life of one session inside the running app - created, folded in,
// restarted. Shared by creation (#32), adoption (#122) and restart (#15, #43);
// model.go stays the state and the message router.

package ui

import (
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// sessionRelaunchMsg carries a session whose held claude has been ended and
// whose replacement process should now start. It exists because the stop half
// runs off the Update goroutine and the start half must not begin until it has
// finished (#15, #43).
type sessionRelaunchMsg struct{ Session registry.Session }

// restartSelected relaunches the focused session's process in place (issue
// #15). It covers a crashed pane and a claude that exited.
//
// The held process is ended first, and the relaunch waits for that. Under a
// detach holder `dtach -A` attaches to a live master and discards the command
// it was handed, so starting without stopping swapped the client and left the
// wedged claude exactly where it was: the key did nothing and said nothing,
// and archiving the row became the only way to end a stuck session (#43).
func (m *Model) restartSelected() tea.Cmd {
	row, ok := m.sidebar.Selected()
	if !ok {
		return nil
	}
	sess := *row.Session
	return m.stopSessionCmd(sess, sessionRelaunchMsg{Session: sess})
}

// relaunch starts the replacement process. The old terminal is closed only
// after the new one starts, so a failed restart never leaves the pane empty;
// the launcher resumes the transcript (#36) so nothing is lost.
func (m *Model) relaunch(sess registry.Session) tea.Cmd {
	w, h := m.ptySize()
	term, err := m.start(sess, w, h)
	if err != nil {
		m.lastErr = fmt.Sprintf("restarting %s: %v", sess.Title, err)
		return nil
	}
	if old := m.terms[sess.ID]; old != nil {
		_ = old.Close()
	}
	m.terms[sess.ID] = term
	// Born at the live size, so no Resize races claude's startup (issue #73).
	// The replacement process gets its own clipboard wait: the old one ended
	// with the terminal it was reading (#212).
	return tea.Batch(term.Init(), m.waitForClipboard(sess.ID))
}

// submitPrompt creates the session. A worktree prompt uses the buffer as both
// the session title and the branch name, and for that prompt the buffer is
// known non-empty: commitEditor leaves it open rather than registering a
// nameless branch. A plain prompt may be blank; the creator registers a
// placeholder title and the first prompt names the session (#127).
func (m *Model) submitPrompt() tea.Cmd {
	branch := ""
	if m.modal.Editor.Worktree {
		branch = m.modal.Editor.Buffer
	}
	m.lastErr = ""
	project := m.SelectedProject()
	title := m.modal.Editor.Buffer
	m.modal = modal{}
	cmd, err := m.addSession(project, title, branch)
	if err != nil {
		slog.Error("creating session",
			"project", project, "title", title, "branch", branch, "err", err)
		m.lastErr = err.Error()
		return nil
	}
	return cmd
}

// addSession registers the session, starts its terminal, and rebuilds the
// sidebar so it is visible and focused immediately (issue #32). A session
// whose terminal will not start is not added: it would be a row you cannot
// focus.
func (m *Model) addSession(project, title, branch string) (tea.Cmd, error) {
	sess, err := m.create(project, title, branch)
	if err != nil {
		return nil, err
	}
	return m.foldInSession(sess)
}

// foldInSession brings a session omatty has just learned about into the running
// app: its terminal, its state, its sidebar row, its tailer.
//
// Shared by creation (#32) and adoption (#122), which differ only in where the
// Session came from - the registry made one, the transcript store named the
// other. Every step here is load-bearing, and a second copy that dropped one
// would fail quietly: no terminal is a row you cannot focus, no tailer is a
// session that never shows status (#33).
func (m *Model) foldInSession(sess registry.Session) (tea.Cmd, error) {
	w, h := m.ptySize()
	term, err := m.start(sess, w, h)
	if err != nil {
		return nil, fmt.Errorf("starting session %s: %w", sess.ID, err)
	}
	m.terms[sess.ID] = term
	m.state.Sessions = append(m.state.Sessions, sess)
	m.sidebar = NewSidebar(SidebarRows(m.state, m.statusMap()))
	if !m.selectSession(sess.ID) {
		slog.Warn("a new session is not in the rebuilt sidebar",
			"session", sess.ID, "project", sess.Project)
	}
	m.tailStart(sess)
	// The new terminal needs its own poll started; the others already have
	// theirs (issue #33). followSession goes with it: this moves the selection,
	// and every other site that moves the selection drags an open review column
	// along. Without it the column kept showing the previous session's diff,
	// title and comments beside the new session's terminal, and r/S/c acted on
	// the wrong session (#21, #95).
	return tea.Batch(term.Init(), m.waitForClipboard(sess.ID), m.followSession()), nil
}

// selectSession moves the cursor onto id, so a freshly created session is the
// one you are looking at, and reports whether the row was there to move to.
//
// The bool is not discarded: a session that was just created and added to the
// rebuilt rows and is still not found means the rebuild dropped it, which is
// the one case addSession would want to hear about.
func (m *Model) selectSession(id string) bool { return m.sidebar.SelectByID(id) }
