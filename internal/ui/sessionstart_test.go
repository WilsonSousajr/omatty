package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// Migration step 5.6a (#653; ADR 0001 pain point 5): `git worktree add` and
// the PTY spawn ran inside Update, so a slow one froze every pane. Submitting
// the prompt now only schedules them: nothing is created or started until the
// command runs, off the Update goroutine.
func TestModel_submittingAPromptCreatesNothingInsideUpdate_issue653(t *testing.T) {
	c, s := &liveCreate{}, &startRecorder{}
	m := ui.NewModel(ui.Deps{State: oneProject(), Terms: map[string]termwrap.Terminal{}, Create: c.fn, Start: s.fn})
	m.Update(ctrl('o'))
	m.Update(key('n'))
	m.Update(key('x'))

	_, cmd := m.Update(special(tea.KeyEnter))

	if c.Calls != 0 || len(s.Started) != 0 {
		t.Fatalf("Update created %d and started %v itself, want neither until the command runs", c.Calls, s.Started)
	}
	settle(m, cmd)
	if c.Calls != 1 || len(s.Started) != 1 {
		t.Errorf("after the command ran: created %d, started %v; want one of each", c.Calls, s.Started)
	}
}

// A restart runs off the Update goroutine too, and a second enter while it is
// on its way must not start a second process: the pane still shows stopped
// until the first lands (#653, #318).
func TestModel_aSecondEnterWhileAStartIsOnItsWayStartsNothing_issue653(t *testing.T) {
	r := newStopRig(t)
	r.stop()

	_, first := r.m.Update(special(tea.KeyEnter))
	_, second := r.m.Update(special(tea.KeyEnter))
	settle(r.m, first)
	settle(r.m, second)

	if len(r.starts.Started) != 1 || r.starts.Started[0] != "s1" {
		t.Errorf("started = %v, want s1 exactly once", r.starts.Started)
	}
}
