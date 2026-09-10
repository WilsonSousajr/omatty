package ui_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/agent"
	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func TestStartTerminals_OnePerSessionInItsOwnDirectory(t *testing.T) {
	var dirs []string
	factory := func(_, _ int, cmd *exec.Cmd) (termwrap.Terminal, error) {
		dirs = append(dirs, cmd.Dir)
		return termwrap.NewFake(""), nil
	}

	terms, err := ui.StartTerminals(
		twoProjectState(), supervisor.NewLauncher(agent.Claude(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, 80, 24, ui.DefaultLeader)

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
		twoProjectState(), supervisor.NewLauncher(agent.Claude(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, 80, 24, ui.DefaultLeader)

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
		emptyState(), supervisor.NewLauncher(agent.Claude(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, 80, 24, ui.DefaultLeader)

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
		twoProjectState(), supervisor.NewLauncher(agent.Claude(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, 80, 24, ui.DefaultLeader)
	if err != nil {
		t.Fatal(err)
	}

	for id, term := range terms {
		if _, ok := term.(*termwrap.Guard); !ok {
			t.Errorf("session %s got %T, want a *termwrap.Guard", id, term)
		}
	}
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

	_, err := ui.StartTerminals(oneSessionState(), supervisor.NewLauncher(agent.Claude(), "claude", "/h.json", t.TempDir(), &detach.Plain{}),
		factory, 140, 40, ui.DefaultLeader)
	if err != nil {
		t.Fatal(err)
	}

	// PaneSize(140, 40) is 112x37, and the PTY is the whole pane now that the
	// title sits in the header row (issue #75, #128, #174).
	if gotW != 112 || gotH != 37 {
		t.Errorf("PTY started at %dx%d, want 112x37 (not the 140x40 window)", gotW, gotH)
	}
}

// oneSessionState has one session, unlike oneProject, so StartTerminals has
// something to start.
func oneSessionState() registry.State {
	return registry.State{
		Projects: []registry.Project{{Name: "p", Root: "/p"}},
		Sessions: []registry.Session{{ID: "s1", Project: "p", Title: "one"}},
	}
}

// heldHolder is a detach.Holder whose Held answers from a set, and whose
// Wrap returns the command unchanged.
type heldHolder struct {
	detach.Plain
	IDs map[string]bool
	Err error
}

func (h *heldHolder) Held(id string) (bool, error) { return h.IDs[id], h.Err }

// The boot asks the holder which sessions are already running, so their
// panes can be nudged to repaint; a failed check reads as fresh (#191).
func TestHeldSessions_AsksTheHolderPerSession_issue191(t *testing.T) {
	l := supervisor.NewLauncher(agent.Claude(), "claude", "/h.json", t.TempDir(), &heldHolder{IDs: map[string]bool{"s2": true}})

	held := ui.HeldSessions(l, twoProjectState())

	if len(held) != 1 || !held["s2"] {
		t.Errorf("HeldSessions() = %v, want only s2", held)
	}
}

func TestHeldSessions_AFailedCheckReadsAsFresh_issue191(t *testing.T) {
	l := supervisor.NewLauncher(agent.Claude(), "claude", "/h.json", t.TempDir(),
		&heldHolder{IDs: map[string]bool{"s1": true}, Err: errors.New("stat exploded")})

	if held := ui.HeldSessions(l, twoProjectState()); len(held) != 0 {
		t.Errorf("HeldSessions() = %v with a failing holder, want none", held)
	}
}
