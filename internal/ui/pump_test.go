package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// emulatorMsg stands in for the output messages bubbleterm emits to re-arm
// its poll loop.
type emulatorMsg struct{ n int }

// Regression, issue #33: Init returned nil, so bubbleterm's self-rescheduling
// poll loop never started and no terminal ever read from its PTY.
func TestModel_InitStartsEverySessionsTerminal_issue33(t *testing.T) {
	m, fakes := modelWithFakes(t)

	cmd := m.Init()

	if cmd == nil {
		t.Error("Init() returned nil; no terminal will ever read its PTY")
	}
	for id, f := range fakes {
		if !f.Inited {
			t.Errorf("terminal %s was never initialised", id)
		}
	}
}

// Regression, issue #33: Update dropped everything that was not a key or a
// resize, so the emulator messages that re-arm the poll never arrived.
func TestModel_UpdateForwardsEmulatorMessages_issue33(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(emulatorMsg{n: 1})

	for id, f := range fakes {
		if len(f.Msgs) != 1 {
			t.Errorf("terminal %s got %d messages, want 1", id, len(f.Msgs))
		}
	}
}

// Every session is pumped, not just the focused one: a background session
// that stops being read blocks on its PTY.
func TestModel_UnfocusedSessionsAreStillPumped_issue33(t *testing.T) {
	m, fakes := modelWithFakes(t)
	if m.Focused() != "s1" {
		t.Fatalf("precondition: Focused() = %q, want s1", m.Focused())
	}

	m.Update(emulatorMsg{n: 2})

	if len(fakes["s2"].Msgs) == 0 || len(fakes["s3"].Msgs) == 0 {
		t.Error("unfocused sessions were not pumped; their PTYs will stall")
	}
}

// Keys must NOT be broadcast: only the focused session may receive them.
func TestModel_KeysAreNotBroadcast_issue33(t *testing.T) {
	m, fakes := modelWithFakes(t)

	press(m, key('a'))

	if len(fakes["s2"].Msgs) != 0 || len(fakes["s3"].Msgs) != 0 {
		t.Error("a keystroke reached an unfocused session; only the focused one may get keys")
	}
}

// A session created at runtime must be initialised too, or its pane is blank.
func TestModel_CreatedSessionsTerminalIsInitialised_issue33(t *testing.T) {
	c, s := &liveCreate{}, &startRecorder{}
	m := ui.NewModel(ui.Deps{State: oneProject(), Terms: map[string]termwrap.Terminal{}, Create: c.fn, Start: s.fn})

	newSession(m, "test")

	if s.Term == nil {
		t.Fatal("no terminal was started")
	}
	if !s.Term.Inited {
		t.Error("the new session's terminal was never initialised; its pane stays blank")
	}
}

var _ tea.Msg = emulatorMsg{}
var _ = registry.State{}
