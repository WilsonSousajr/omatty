package app_test

import (
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// Migration step 5.6a (#653; ADR 0001 pain point 5): `git worktree add` and
// the PTY spawn ran inside Update, so a slow one froze every pane. A create
// that does not return must not hold Update: the enter comes back at once, and
// the session arrives when the create does.
func TestModel_aSlowCreateDoesNotHoldUpdate_issue653(t *testing.T) {
	release := make(chan struct{})
	s := &startRecorder{}
	create := func(project, title, _ string, _ bool) (session.Session, error) {
		<-release
		return session.Session{ID: "new-id", Project: project, Title: title}, nil
	}
	m := app.NewModel(app.Deps{State: oneProject(), Terms: map[string]terminal.Terminal{}, Create: create, Start: s.fn})
	m.Update(ctrl('o'))
	m.Update(key('n'))
	m.Update(key('x'))

	returned := make(chan tea.Cmd, 1)
	go func() { _, cmd := m.Update(special(tea.KeyEnter)); returned <- cmd }()
	var cmd tea.Cmd
	select {
	case cmd = <-returned:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("Update waited for the create to finish")
	}
	close(release)
	settle(m, cmd)
	if len(s.Started) != 1 || m.Selected() != "new-id" {
		t.Errorf("after the create landed: started %v, selected %q; want new-id started and selected", s.Started, m.Selected())
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
