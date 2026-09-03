package ui

import (
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// StartTerminals launches one embedded terminal per registered session, keyed
// by session id. w and h are the WINDOW size; each PTY is born at the pane
// size, so claude paints at the right width from its first frame instead of
// racing a later resize (issue #51).
func StartTerminals(
	st registry.State, l *supervisor.Launcher, f termwrap.Factory, w, h int,
) (map[string]termwrap.Terminal, error) {
	// The review column is closed at birth, so the terminal gets the full
	// width beside the sidebar (#21).
	pw, ph := PTYSize(w, h, false)
	terms := make(map[string]termwrap.Terminal, len(st.Sessions))
	for _, sess := range st.Sessions {
		term, err := l.Start(f, sess, pw, ph)
		if err != nil {
			return nil, fmt.Errorf("ui: starting terminal for session %s: %w", sess.ID, err)
		}
		// Invariant 6: one emulator panic must not take down the app.
		terms[sess.ID] = termwrap.NewGuard(term)
	}
	return terms, nil
}

// Run starts every session's terminal, the status watcher, and the TUI, and
// runs until the user quits. create is called when the operator finishes a
// new-session prompt; the model starts that session's terminal itself
// through the same launcher. diff loads a session's changes for the review
// column, so ui never reaches git itself (invariant 4, #21).
func Run(
	home string, st registry.State, l *supervisor.Launcher, f termwrap.Factory, w, h int,
	create CreateFunc, diff DiffFunc,
) error {
	terms, err := StartTerminals(st, l, f, w, h)
	if err != nil {
		return err
	}
	defer closeTerminals(terms)
	watch := watcher.Start(home, st.Sessions, time.Now)
	defer watch.Close()
	model := NewModel(Deps{
		State: st, Terms: terms, Create: create, Start: guardedStarter(l, f), Diff: diff,
		Events: watch.Events(), Clock: time.Now, Notifier: notify.New(), TailStart: watch.Add,
	})
	return runProgram(model, len(terms))
}

// closeTerminals ends every claude process on the way out (issue #72). The
// map is the one the model adds runtime sessions to, so those close too.
// Until now the OS closed the PTY masters at exit, which is neither a
// guarantee nor omatty's decision.
func closeTerminals(terms map[string]termwrap.Terminal) {
	for id, t := range terms {
		if err := t.Close(); err != nil {
			slog.Warn("closing a terminal on exit", "session", id, "err", err)
		}
	}
}

// runProgram runs the bubbletea program to completion.
func runProgram(model *Model, sessions int) error {
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("ui: running the program with %d sessions: %w", sessions, err)
	}
	return nil
}

// guardedStarter starts a session's terminal wrapped in a panic guard
// (invariant 6). The model passes the live pane size on every call.
func guardedStarter(l *supervisor.Launcher, f termwrap.Factory) StartFunc {
	return func(sess registry.Session, w, h int) (termwrap.Terminal, error) {
		term, err := l.Start(f, sess, w, h)
		if err != nil {
			return nil, err
		}
		return termwrap.NewGuard(term), nil
	}
}
