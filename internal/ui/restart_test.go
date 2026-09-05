package ui_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func modelWithStarter(t *testing.T, s *startRecorder) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	return ui.NewModel(ui.Deps{State: twoProjectState(), Terms: terms, Create: noCreate, Start: s.fn}), fakes
}

// Regression, issue #15: the crash frame has told the operator to press
// ctrl+o r since #13, and nothing was bound to it.
func TestModel_ctrlOrRestartsTheFocusedSession_issue15(t *testing.T) {
	s := &startRecorder{}
	m, fakes := modelWithStarter(t, s)
	old := fakes["s1"]

	press(m, ctrl('o'))
	_, cmd := m.Update(key('r'))
	// The stop half runs off the Update goroutine and the start half waits for
	// it, so the restart is not finished until its command has run (#43).
	settle(m, cmd)

	if len(s.Started) != 1 || s.Started[0] != "s1" {
		t.Fatalf("started %v, want exactly [s1]", s.Started)
	}
	if !old.Closed {
		t.Error("the old terminal was not closed; its process would leak")
	}
	if !s.Term.Inited {
		t.Error("the new terminal was not initialised; its pane would stay blank (#33)")
	}
	if cmd == nil {
		t.Error("restart returned no command; the held claude is never stopped")
	}
	if got := m.Selected(); got != "s1" {
		t.Errorf("Focused() = %q after restart, want s1 unchanged", got)
	}
	if !strings.Contains(m.View().Content, "terminal for") {
		t.Errorf("the restarted terminal is not the one rendered:\n%s", m.View().Content)
	}
}

func TestModel_ctrlOrWithNoSessionIsHarmless_issue15(t *testing.T) {
	s := &startRecorder{}
	m := ui.NewModel(ui.Deps{State: registry.State{}, Terms: map[string]termwrap.Terminal{}, Create: noCreate, Start: s.fn})

	press(m, ctrl('o'))
	pressAndSettle(m, key('r'))

	if len(s.Started) != 0 {
		t.Errorf("started %v with no session focused, want nothing", s.Started)
	}
	if strings.Contains(m.View().Content, "error:") {
		t.Errorf("an error was shown for a harmless no-op:\n%s", m.View().Content)
	}
}

// Never leave the pane empty: if the restart fails, the old terminal stays.
func TestModel_ctrlOrStartFailureSurfacesAndKeepsTheOldTerminal_issue15(t *testing.T) {
	s := &startRecorder{Err: errors.New("pty exhausted")}
	m, fakes := modelWithStarter(t, s)

	press(m, ctrl('o'))
	pressAndSettle(m, key('r'))

	if !strings.Contains(m.View().Content, "pty exhausted") {
		t.Errorf("the failure is not surfaced:\n%s", m.View().Content)
	}
	if fakes["s1"].Closed {
		t.Error("the old terminal was closed even though the replacement failed to start")
	}
	if !strings.Contains(m.View().Content, "session one") {
		t.Error("the old terminal is no longer rendered")
	}
}

var _ tea.Msg = tea.KeyPressMsg{}

// Regression, issue #73: restart birthed at the frozen size and resized
// afterwards, the very race #51 removed for startup.
func TestModel_RestartBirthsAtTheCurrentPTYSize_issue73(t *testing.T) {
	s := &startRecorder{}
	m, _ := modelWithStarter(t, s)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})

	press(m, ctrl('o'))
	pressAndSettle(m, key('r'))

	if s.W != 170 || s.H != 56 {
		t.Errorf("restarted at %dx%d, want PTYSize(200,60) = 170x56", s.W, s.H)
	}
}
