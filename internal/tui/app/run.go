package app

import (
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// StartTerminals launches an embedded terminal for each session in want,
// keyed by session id. w and h are the WINDOW size; each PTY is born at the
// pane size, so claude paints at the right width from its first frame instead
// of racing a later resize (issue #51).
//
//	terms := app.StartTerminals(st, app.SessionsToStart(lazy, st, held), l, terminal.Start, w, h, leader)
//
// A session left out - not wanted, or failing to start - has no terminal,
// which its pane shows as stopped with enter to start it (#318). One failed
// start used to abort the whole boot, so a single bad session kept every
// other one from opening; it is logged and skipped instead (#317).
func StartTerminals(
	st sessions.State, want map[string]bool, l *sessions.Launcher, f terminal.Factory, w, h int, leader string,
) map[string]terminal.Terminal {
	// The review column is closed at birth, so the terminal gets the full
	// width beside the sidebar (#21).
	pw, ph := PTYSize(w, h, false)
	terms := make(map[string]terminal.Terminal, len(want))
	for _, sess := range st.Sessions {
		if !want[sess.ID] {
			continue
		}
		term, err := startSession(l, f, sess, pw, ph)
		if err != nil {
			slog.Warn("starting a session's terminal at boot; its pane shows it stopped",
				"session", sess.ID, "dir", sess.Dir, "err", err)
			continue
		}
		// Invariant 6: one emulator panic must not take down the app.
		terms[sess.ID] = terminal.NewGuard(term, leader+" r")
	}
	return terms
}

// SessionsToStart is the boot policy in one place (#317). Under lazy start it
// is exactly the sessions a holder is already keeping alive: attaching one
// costs a dtach client and a PTY, while not attaching frees nothing - its
// claude runs on either way - and would leave #191's repaint nudge with no
// terminal to repaint. Every other session waits for enter. Without dtach
// nothing is held, so a lazy boot starts none, which is right: nothing
// survived the quit, so each was going to cold-start anyway, and now does so
// on demand. lazy_start = false starts every session, as before.
//
//	want := app.SessionsToStart(cfg.Sessions.LazyStart, st, held)
func SessionsToStart(lazy bool, st sessions.State, held map[string]bool) map[string]bool {
	if lazy {
		return held
	}
	all := make(map[string]bool, len(st.Sessions))
	for _, sess := range st.Sessions {
		all[sess.ID] = true
	}
	return all
}

// HeldSessions is the set of sessions whose claude is already running from
// an earlier omatty, so the boot can tell a pane that will come back blank
// from one that will paint itself (#191). A failed liveness check is logged
// and reads as fresh: the pane then behaves as it did before this existed.
//
//	held := app.HeldSessions(launcher, state)
func HeldSessions(l *sessions.Launcher, st sessions.State) map[string]bool {
	held := map[string]bool{}
	for _, sess := range st.Sessions {
		ok, err := l.Reattaching(sess.ID)
		if err != nil {
			slog.Warn("checking whether a session is held", "session", sess.ID, "err", err)
			continue
		}
		if ok {
			held[sess.ID] = true
		}
	}
	return held
}

// LeaderOr is the configured leader, or DefaultLeader for an empty one (#44).
//
//	leader := app.LeaderOr(cfg.Leader)
func LeaderOr(leader string) string {
	if leader == "" {
		return DefaultLeader
	}
	return leader
}

// CloseTerminals closes every PTY on the way out (issue #72). The map is the
// one the model adds runtime sessions to, so those close too. Until #72 the OS
// closed the masters at exit, which is neither a guarantee nor omatty's
// decision.
//
// What this ends depends on the holder, and the distinction is the whole point
// of M6: under Plain it closes the PTY and the claude on the other side of it
// dies with the SIGHUP, exactly as before; under a detach holder it closes only
// the dtach *client*, and the master and its claude go on running. Archiving is
// the one place omatty ends a claude on purpose, and dropSession is where that
// is written (#43).
//
//	defer app.CloseTerminals(terms)
func CloseTerminals(terms map[string]terminal.Terminal) {
	for id, t := range terms {
		if err := t.Close(); err != nil {
			slog.Warn("closing a terminal on exit", "session", id, "err", err)
		}
	}
}

// RunProgram runs the bubbletea program to completion. cmd/omatty, the one
// composition root since migration step 5.10 (#653), builds the model and
// owns everything that lives as long as it.
//
//	err := app.RunProgram(app.NewModel(deps), len(terms))
func RunProgram(model *Model, sessions int) error {
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("ui: running the program with %d sessions: %w", sessions, err)
	}
	return nil
}

// GuardedStarter starts a session's terminal wrapped in a panic guard
// (invariant 6). The model passes the live pane size on every call.
//
//	deps.Start = app.GuardedStarter(launcher, terminal.Start, leader)
func GuardedStarter(l *sessions.Launcher, f terminal.Factory, leader string) StartFunc {
	return func(sess sessions.Session, w, h int) (terminal.Terminal, error) {
		term, err := startSession(l, f, sess, w, h)
		if err != nil {
			return nil, err
		}
		return terminal.NewGuard(term, leader+" r"), nil
	}
}

// startSession launches a session's process inside a w by h embedded
// terminal: the session service says what runs, the terminal spawns it (ADR
// 0001, "Starting a session"). It was supervisor's Launcher.Start until
// migration step 5.5 (#653) moved the launcher into the service, which may not
// name a terminal.
func startSession(
	l *sessions.Launcher, f terminal.Factory, sess sessions.Session, w, h int,
) (terminal.Terminal, error) {
	launch, err := l.Launch(sess)
	if err != nil {
		return nil, err
	}
	term, err := f(w, h, launch)
	if err != nil {
		return nil, fmt.Errorf("supervisor: starting session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	return term, nil
}
