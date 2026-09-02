package ui

import (
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// eventBuffer sizes the channel between the watcher and the UI. A short burst
// (a session finishing several tools) must not block a hook.
const eventBuffer = 64

// StartTerminals launches one embedded terminal per registered session, keyed
// by session id. w and h are the WINDOW size; each PTY is born at the pane
// size, so claude paints at the right width from its first frame instead of
// racing a later resize (issue #51).
func StartTerminals(
	st registry.State, l *supervisor.Launcher, f termwrap.Factory, w, h int,
) (map[string]termwrap.Terminal, error) {
	pw, ph := PaneSize(w, h)
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

// Run starts every session's terminal and runs the TUI until the user quits.
// create is called when the operator finishes a new-session prompt; the model
// starts that session's terminal itself through the same launcher.
func Run(
	home string, st registry.State, l *supervisor.Launcher, f termwrap.Factory, w, h int,
	create CreateFunc,
) error {
	terms, err := StartTerminals(st, l, f, w, h)
	if err != nil {
		return err
	}
	start := guardedStarter(l, f, w, h)
	events := make(chan watcher.Event, eventBuffer)
	closeListener := startListener(home, events)
	defer closeListener()
	model, closeTailers := wireStatus(home, st, terms, create, start, events)
	defer closeTailers()
	return runProgram(model, len(terms))
}

// startListener opens the hook socket, or logs and degrades to tailer-only if
// it cannot bind (issue #49). The listener is the low-latency source; the
// tailer is the source of truth, so a lost socket costs only the instant
// hook-driven "waiting" glyph, never the whole app.
func startListener(home string, events chan<- watcher.Event) func() {
	l, err := watcher.Listen(paths.HookSocket(home), events, time.Now)
	if err != nil {
		slog.Warn("hook socket unavailable; status comes from the transcript only", "err", err)
		return func() {}
	}
	return func() { _ = l.Close() }
}

// runProgram runs the bubbletea program to completion.
func runProgram(model *Model, sessions int) error {
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("ui: running the program with %d sessions: %w", sessions, err)
	}
	return nil
}

// wireStatus builds the model and connects it to the live status stream: a
// tailer per session now, and one per session created at runtime.
func wireStatus(
	home string, st registry.State, terms map[string]termwrap.Terminal,
	create CreateFunc, start StartFunc, events chan watcher.Event,
) (*Model, func()) {
	tail := tailStarter(home, events)
	tailers := StartTailers(st, tail)
	model := NewModel(st, terms, create, start)
	model.SetEvents(events, time.Now)
	model.SetNotifier(notify.Osascript{})
	model.SetTailStarter(func(sess registry.Session) { tailers = append(tailers, tail(sess)) })
	return model, func() { closeAll(tailers) }
}

// guardedStarter starts a session's terminal wrapped in a panic guard
// (invariant 6). Used both for the initial sessions and for one created at
// runtime.
func guardedStarter(l *supervisor.Launcher, f termwrap.Factory, w, h int) StartFunc {
	pw, ph := PaneSize(w, h)
	return func(sess registry.Session) (termwrap.Terminal, error) {
		term, err := l.Start(f, sess, pw, ph)
		if err != nil {
			return nil, err
		}
		return termwrap.NewGuard(term), nil
	}
}

// TailFunc starts a transcript tailer for one session.
type TailFunc func(sess registry.Session) *watcher.Tailer

// tailStarter binds the home and event sink a tailer needs.
func tailStarter(home string, events chan<- watcher.Event) TailFunc {
	return func(sess registry.Session) *watcher.Tailer {
		path := paths.Transcript(home, sess.Dir, sess.ID)
		return watcher.Tail(sess.ID, path, events, time.Now, time.Second)
	}
}

// StartTailers starts one tailer per registered session.
func StartTailers(st registry.State, tail TailFunc) []*watcher.Tailer {
	tailers := make([]*watcher.Tailer, 0, len(st.Sessions))
	for _, sess := range st.Sessions {
		tailers = append(tailers, tail(sess))
	}
	return tailers
}

func closeAll(tailers []*watcher.Tailer) {
	for _, t := range tailers {
		t.Close()
	}
}

// WireStatusForTest exposes wireStatus to the package's external tests.
func WireStatusForTest(
	home string, st registry.State, terms map[string]termwrap.Terminal,
	create CreateFunc, start StartFunc, events chan watcher.Event,
) (*Model, func()) {
	return wireStatus(home, st, terms, create, start, events)
}
