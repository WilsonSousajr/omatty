// Creating a session and starting its process, off the Update goroutine (ADR
// 0001 pain point 5; migration step 5.6a, #653). `git worktree add` and a PTY
// spawn both ran inside Update, so a slow disk, a hung git or a large checkout
// froze every pane until they returned. Each is now a command whose answer
// comes back as a message and is folded in here.

package ui

import (
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// sessionCreatedMsg is the registry's answer to a new-session prompt: the
// session it wrote, or why there is none.
type sessionCreatedMsg struct {
	sess                   sessions.Session
	err                    error
	project, title, branch string
}

// sessionStartedMsg is a session's process, started or not. restart says
// whether it replaces a terminal (ctrl+o r, enter on a stopped pane) or brings
// a session into the app for the first time (creation, adoption).
type sessionStartedMsg struct {
	sess    sessions.Session
	term    termwrap.Terminal
	err     error
	restart bool
}

// createCmd registers a session off the Update goroutine.
//
// It is a registry write, so it joins the writer's queue behind every write
// asked for before it (5.6b, #653): a create racing a rename would otherwise
// load state.json, and save it, across the rename's own load and save.
func (m *Model) createCmd(project, title, branch string, worktree bool) tea.Cmd {
	create := m.create
	var sess sessions.Session
	result := m.writes.submit(func() error {
		var err error
		sess, err = create(project, title, branch, worktree)
		return err
	})
	return func() tea.Msg {
		err := <-result
		return sessionCreatedMsg{sess: sess, err: err, project: project, title: title, branch: branch}
	}
}

// onSessionCreated starts the session the registry wrote, or says why there
// is none.
func (m *Model) onSessionCreated(msg sessionCreatedMsg) tea.Cmd {
	if msg.err != nil {
		slog.Error("creating session",
			"project", msg.project, "title", msg.title, "branch", msg.branch, "err", msg.err)
		m.lastErr = msg.err.Error()
		return nil
	}
	return m.startCmd(msg.sess, false)
}

// startCmd starts a session's process off the Update goroutine, at the pane's
// live size so no Resize races claude's startup (#73). A session already on
// its way is left to it.
func (m *Model) startCmd(sess sessions.Session, restart bool) tea.Cmd {
	if m.starting[sess.ID] {
		return nil
	}
	if m.starting == nil {
		m.starting = map[string]bool{}
	}
	m.starting[sess.ID] = true
	w, h := m.ptySize()
	start := m.start
	return func() tea.Msg {
		term, err := start(sess, w, h)
		return sessionStartedMsg{sess: sess, term: term, err: err, restart: restart}
	}
}

// onSessionStarted folds a started process in: as a replacement terminal, or
// as a session new to the app.
func (m *Model) onSessionStarted(msg sessionStartedMsg) tea.Cmd {
	delete(m.starting, msg.sess.ID)
	if msg.restart {
		return m.restarted(msg)
	}
	return m.foldedIn(msg)
}

// restarted puts a replacement terminal in place. The old one is closed only
// now that the new one runs, so a failed restart never leaves the pane empty.
// A session archived while its process was starting has no pane to take it,
// so the new terminal is closed rather than left running unseen.
func (m *Model) restarted(msg sessionStartedMsg) tea.Cmd {
	sess := msg.sess
	if msg.err != nil {
		m.lastErr = fmt.Sprintf("restarting %s: %v", sess.Title, msg.err)
		return nil
	}
	if !m.knownSession(sess.ID) {
		_ = msg.term.Close()
		return nil
	}
	if old := m.terms[sess.ID]; old != nil {
		_ = old.Close()
	}
	m.terms[sess.ID] = msg.term
	m.markActive(sess.ID) // a fresh process is not idle (#319)
	// The replacement process gets its own clipboard wait: the old one ended
	// with the terminal it was reading (#212).
	return tea.Batch(msg.term.Init(), m.waitForClipboard(sess.ID))
}

// foldedIn adds a started session to the running app, or says why it could
// not start. A session whose terminal will not start is not added: it would
// be a row you cannot focus (#32).
func (m *Model) foldedIn(msg sessionStartedMsg) tea.Cmd {
	if msg.err != nil {
		err := fmt.Errorf("starting session %s: %w", msg.sess.ID, msg.err)
		slog.Error("starting a session", "session", msg.sess.ID, "dir", msg.sess.Dir, "err", err)
		m.lastErr = err.Error()
		return nil
	}
	return m.foldInSession(msg.sess, msg.term)
}
