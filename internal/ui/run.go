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

// RunDeps is everything Run needs: the runtime plumbing, plus the functions
// injected so ui never reaches git or the registry store itself (invariant 4).
//
// A struct rather than a parameter list because M4 adds one injected function
// per feature - rename here (#41), archive and discovery after it - and the
// list was already at eight (#41).
//
//	ui.Run(ui.RunDeps{Home: home, State: st, Launch: l, Factory: termwrap.Start,
//	        Width: w, Height: h, Create: create, Diff: diff, Files: files, Rename: rename})
type RunDeps struct {
	Home    string
	State   registry.State
	Launch  *supervisor.Launcher
	Factory termwrap.Factory
	Width   int
	Height  int
	// Create is called when the operator finishes a new-session prompt; the
	// model starts that session's terminal itself through the same launcher.
	Create CreateFunc
	// Diff loads a session's changes for the review column and Files lists its
	// worktree for the tree (#21, #24).
	Diff  DiffFunc
	Files ListFilesFunc
	// Rename persists a session's new title (#41).
	Rename RenameFunc
	// Archive drops a session from the registry and RemoveWorktree deletes its
	// worktree (#40). The tailer is stopped through the Watch this owns.
	Archive        ArchiveFunc
	RemoveWorktree RemoveWorktreeFunc
	// Discover proposes repositories to register and AddProject registers one
	// (#91).
	Discover   DiscoverFunc
	AddProject AddProjectFunc
}

// Run starts every session's terminal, the status watcher, and the TUI, and
// runs until the user quits.
func Run(d RunDeps) error {
	terms, err := StartTerminals(d.State, d.Launch, d.Factory, d.Width, d.Height)
	if err != nil {
		return err
	}
	defer closeTerminals(terms)
	watch := watcher.Start(d.Home, d.State.Sessions, time.Now)
	defer watch.Close()
	model := NewModel(Deps{
		State: d.State, Terms: terms, Create: d.Create, Start: guardedStarter(d.Launch, d.Factory),
		Diff: d.Diff, Files: d.Files, Rename: d.Rename,
		Archive: d.Archive, RemoveWorktree: d.RemoveWorktree,
		Discover: d.Discover, AddProject: d.AddProject,
		Events: watch.Events(), Clock: time.Now, Notifier: notify.New(),
		TailStart: watch.Add, TailStop: watch.Remove,
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
