package ui_test

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func TestStartTerminals_OnePerSessionInItsOwnDirectory(t *testing.T) {
	var dirs []string
	factory := func(_, _ int, cmd *exec.Cmd) (termwrap.Terminal, error) {
		dirs = append(dirs, cmd.Dir)
		return termwrap.NewFake(""), nil
	}

	terms, err := ui.StartTerminals(
		twoProjectState(), supervisor.NewLauncher("claude", "/h.json", t.TempDir()), factory, 80, 24)

	if err != nil {
		t.Fatalf("StartTerminals() error = %v, want nil", err)
	}
	if len(terms) != 3 {
		t.Errorf("started %d terminals, want 3", len(terms))
	}
	if len(dirs) != 3 {
		t.Errorf("factory called %d times, want 3", len(dirs))
	}
	for _, id := range []string{"s1", "s2", "s3"} {
		if terms[id] == nil {
			t.Errorf("no terminal registered for session %q", id)
		}
	}
}

func TestStartTerminals_FailureNamesTheSession(t *testing.T) {
	factory := func(int, int, *exec.Cmd) (termwrap.Terminal, error) {
		return nil, errors.New("pty exhausted")
	}

	_, err := ui.StartTerminals(
		twoProjectState(), supervisor.NewLauncher("claude", "/h.json", t.TempDir()), factory, 80, 24)

	if err == nil {
		t.Fatal("StartTerminals() returned nil after a factory failure, want an error")
	}
	if !strings.Contains(err.Error(), "s1") {
		t.Errorf("error %q does not name the offending session", err)
	}
}

func TestStartTerminals_EmptyRegistryStartsNothing(t *testing.T) {
	called := 0
	factory := func(int, int, *exec.Cmd) (termwrap.Terminal, error) {
		called++
		return termwrap.NewFake(""), nil
	}

	terms, err := ui.StartTerminals(
		emptyState(), supervisor.NewLauncher("claude", "/h.json", t.TempDir()), factory, 80, 24)

	if err != nil {
		t.Fatalf("StartTerminals() error = %v, want nil", err)
	}
	if len(terms) != 0 || called != 0 {
		t.Errorf("started %d terminals with %d factory calls, want 0 and 0", len(terms), called)
	}
}

// Invariant 6: every started terminal is guarded, so one emulator panic
// cannot take down the app.
func TestStartTerminals_WrapsEveryTerminalInAGuard(t *testing.T) {
	factory := func(int, int, *exec.Cmd) (termwrap.Terminal, error) {
		return termwrap.NewFake(""), nil
	}

	terms, err := ui.StartTerminals(
		twoProjectState(), supervisor.NewLauncher("claude", "/h.json", t.TempDir()), factory, 80, 24)
	if err != nil {
		t.Fatal(err)
	}

	for id, term := range terms {
		if _, ok := term.(*termwrap.Guard); !ok {
			t.Errorf("session %s got %T, want a *termwrap.Guard", id, term)
		}
	}
}

func TestStartTailers_OnePerSession_issue19(t *testing.T) {
	var started []string
	tail := func(sess registry.Session) *watcher.Tailer {
		started = append(started, sess.ID)
		// A tailer over a path that will never exist is harmless; Poll no-ops.
		return watcher.Tail(sess.ID, filepath.Join(t.TempDir(), sess.ID), make(chan watcher.Event, 1), time.Now, time.Hour)
	}

	tailers := ui.StartTailers(twoProjectState(), tail)
	for _, tl := range tailers {
		tl.Close()
	}

	if len(started) != 3 || len(tailers) != 3 {
		t.Errorf("started %v (%d tailers), want one per session (3)", started, len(tailers))
	}
}

func TestWireStatus_StartsATailerPerSessionAndClosesThemAll_issue19(t *testing.T) {
	home := t.TempDir()
	events := make(chan watcher.Event, 8)
	terms := map[string]termwrap.Terminal{}

	m, closeTailers := ui.WireStatusForTest(home, twoProjectState(), terms,
		noCreate, noStart, events)
	defer closeTailers()

	if m == nil {
		t.Fatal("wireStatus returned no model")
	}
	// A session created at runtime must also get a tailer; closeTailers must
	// not panic on the extended slice.
	closeTailers()
}

// Regression, issue #51: the PTY was born at the raw window size (in practice
// the 80x24 default), so claude painted at the wrong width and never reflowed.
// StartTerminals must start each terminal at PaneSize(window).
func TestStartTerminals_BirthsThePTYAtThePaneSize_issue51(t *testing.T) {
	var gotW, gotH int
	factory := func(w, h int, _ *exec.Cmd) (termwrap.Terminal, error) {
		gotW, gotH = w, h
		return termwrap.NewFake(""), nil
	}

	_, err := ui.StartTerminals(oneProject1(), supervisor.NewLauncher("claude", "/h.json", t.TempDir()),
		factory, 140, 40)
	if err != nil {
		t.Fatal(err)
	}

	wantW, wantH := ui.PaneSize(140, 40)
	if gotW != wantW || gotH != wantH {
		t.Errorf("PTY started at %dx%d, want the pane size %dx%d (not the 140x40 window)",
			gotW, gotH, wantW, wantH)
	}
}

func oneProject1() registry.State {
	return registry.State{
		Projects: []registry.Project{{Name: "p", Root: "/p"}},
		Sessions: []registry.Session{{ID: "s1", Project: "p", Title: "one"}},
	}
}
