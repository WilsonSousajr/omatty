package app_test

import (
	"errors"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"sort"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// claudeProfile stands in for cmd's catalog entry, which this package cannot
// import (ADR 0001, migration step 5.2b, #653). Starting a terminal needs
// only the command template and where to look for a transcript.
func claudeProfile() agent.Profile {
	return agent.Profile{Name: "claude", DefaultBin: "claude", Command: agent.ClaudeCommand, TranscriptPath: paths.Transcript}
}

func TestStartTerminals_OnePerSessionInItsOwnDirectory(t *testing.T) {
	var dirs []string
	factory := func(_, _ int, cmd session.Launch) (terminal.Terminal, error) {
		dirs = append(dirs, cmd.Dir)
		return terminal.NewFake(""), nil
	}

	terms := app.StartTerminals(
		twoProjectState(), every(twoProjectState()), sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, 80, 24, app.DefaultLeader)

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

// Regression, issue #317: one failed start aborted the whole boot, so a
// single bad session kept every other one from opening. It is now left out
// of the map - which the pane shows as stopped, with enter to retry - and
// the rest start.
func TestStartTerminals_AFailedStartLeavesOnlyThatSessionStopped_issue317(t *testing.T) {
	calls := 0
	factory := func(int, int, session.Launch) (terminal.Terminal, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("pty exhausted")
		}
		return terminal.NewFake(""), nil
	}

	terms := app.StartTerminals(
		twoProjectState(), every(twoProjectState()), sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, 80, 24, app.DefaultLeader)

	if terms["s1"] != nil || terms["s2"] == nil || terms["s3"] == nil {
		t.Errorf("started %v, want s2 and s3 with s1 left stopped", keysOf(terms))
	}
}

// Lazy start hands StartTerminals only the sessions a holder already keeps
// alive; the others are not started at all (#317).
func TestStartTerminals_StartsOnlyTheWantedSessions_issue317(t *testing.T) {
	calls := 0
	factory := func(int, int, session.Launch) (terminal.Terminal, error) {
		calls++
		return terminal.NewFake(""), nil
	}

	terms := app.StartTerminals(
		twoProjectState(), map[string]bool{"s2": true}, sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, 80, 24, app.DefaultLeader)

	if len(terms) != 1 || terms["s2"] == nil || calls != 1 {
		t.Errorf("started %v with %d factory calls, want only s2 and one call", keysOf(terms), calls)
	}
}

// every is the want-set that starts every session, as lazy_start = false does.
func every(st session.State) map[string]bool {
	ids := map[string]bool{}
	for _, sess := range st.Sessions {
		ids[sess.ID] = true
	}
	return ids
}

func keysOf(terms map[string]terminal.Terminal) []string {
	ids := make([]string, 0, len(terms))
	for id := range terms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func TestStartTerminals_EmptyRegistryStartsNothing(t *testing.T) {
	called := 0
	factory := func(int, int, session.Launch) (terminal.Terminal, error) {
		called++
		return terminal.NewFake(""), nil
	}

	terms := app.StartTerminals(
		emptyState(), every(emptyState()), sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, 80, 24, app.DefaultLeader)

	if len(terms) != 0 || called != 0 {
		t.Errorf("started %d terminals with %d factory calls, want 0 and 0", len(terms), called)
	}
}

// Invariant 6: every started terminal is guarded, so one emulator panic
// cannot take down the app.
func TestStartTerminals_WrapsEveryTerminalInAGuard(t *testing.T) {
	factory := func(int, int, session.Launch) (terminal.Terminal, error) {
		return terminal.NewFake(""), nil
	}

	terms := app.StartTerminals(
		twoProjectState(), every(twoProjectState()), sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, 80, 24, app.DefaultLeader)

	for id, term := range terms {
		if _, ok := term.(*terminal.Guard); !ok {
			t.Errorf("session %s got %T, want a *terminal.Guard", id, term)
		}
	}
}

// Regression, issue #51: the PTY was born at the raw window size (in practice
// the 80x24 default), so claude painted at the wrong width and never reflowed.
// StartTerminals must start each terminal at PaneSize(window).
func TestStartTerminals_BirthsThePTYAtThePaneSize_issue51(t *testing.T) {
	var gotW, gotH int
	factory := func(w, h int, _ session.Launch) (terminal.Terminal, error) {
		gotW, gotH = w, h
		return terminal.NewFake(""), nil
	}

	app.StartTerminals(oneSessionState(), every(oneSessionState()), sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), &detach.Plain{}),
		factory, 140, 40, app.DefaultLeader)

	// PaneSize(140, 40) is 112x37, and the PTY is the whole pane now that the
	// title sits in the header row (issue #75, #128, #174).
	if gotW != 111 || gotH != 37 {
		t.Errorf("PTY started at %dx%d, want 111x37 (not the 140x40 window)", gotW, gotH)
	}
}

// oneSessionState has one session, unlike oneProject, so StartTerminals has
// something to start.
func oneSessionState() session.State {
	return session.State{
		Projects: []session.Project{{Name: "p", Root: "/p"}},
		Sessions: []session.Session{{ID: "s1", Project: "p", Title: "one"}},
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
	l := sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), &heldHolder{IDs: map[string]bool{"s2": true}})

	held := app.HeldSessions(l, twoProjectState())

	if len(held) != 1 || !held["s2"] {
		t.Errorf("HeldSessions() = %v, want only s2", held)
	}
}

func TestHeldSessions_AFailedCheckReadsAsFresh_issue191(t *testing.T) {
	l := sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(),
		&heldHolder{IDs: map[string]bool{"s1": true}, Err: errors.New("stat exploded")})

	if held := app.HeldSessions(l, twoProjectState()); len(held) != 0 {
		t.Errorf("HeldSessions() = %v with a failing holder, want none", held)
	}
}
