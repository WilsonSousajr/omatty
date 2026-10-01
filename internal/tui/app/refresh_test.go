package app_test

import (
	"errors"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// liveCreate is a named fake standing in for sessions.AddSession: it returns
// the session it would have persisted.
type liveCreate struct {
	Next    session.Session
	Err     error
	Calls   int
	Project string
}

func (l *liveCreate) fn(req sessions.NewSession) (session.Session, error) {
	l.Calls++
	l.Project = req.Project
	if l.Err != nil {
		return session.Session{}, l.Err
	}
	l.Next = session.Session{
		ID: "new-id", Project: req.Project, Title: req.Title,
		Branch: req.Branch, Worktree: req.Worktree,
	}
	return l.Next, nil
}

// startRecorder stands in for launching a real claude process. W and H
// record the size the last start asked for (issue #73).
type startRecorder struct {
	Started []string
	W, H    int
	Err     error
	Term    *terminal.Fake
}

func (s *startRecorder) fn(sess session.Session, w, h int) (terminal.Terminal, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	s.Started = append(s.Started, sess.ID)
	s.W, s.H = w, h
	s.Term = terminal.NewFake("terminal for " + sess.Title)
	return s.Term, nil
}

func oneProject() session.State {
	return session.State{Projects: []session.Project{{Name: "omatty", Root: "/p/omatty"}}}
}

func newSession(m *app.Model, title string) {
	m.Update(ctrl('o'))
	m.Update(key('n'))
	for _, r := range title {
		m.Update(key(r))
	}
	// Creating and starting run off the Update goroutine since #653, so the
	// enter's work is settled as the runtime would run it.
	pressAndSettle(m, special(tea.KeyEnter))
}

// Regression, issue #32: the sidebar was built once in NewModel and never
// rebuilt, so a session created in the TUI was real on disk and invisible on
// screen - the display fell back to "no sessions".
func TestModel_createdSessionAppearsImmediately_issue32(t *testing.T) {
	c, s := &liveCreate{}, &startRecorder{}
	m := app.NewModel(app.Deps{State: oneProject(), Terms: map[string]terminal.Terminal{}, Create: c.fn, Start: s.fn})

	newSession(m, "test")

	got := m.View().Content
	if strings.Contains(got, "no sessions") {
		t.Errorf("still showing the empty state after creating a session:\n%s", got)
	}
	if !strings.Contains(got, "test") {
		t.Errorf("the new session is not listed:\n%s", got)
	}
}

// A session with no terminal would be a row you cannot focus.
func TestModel_createdSessionGetsATerminal_issue32(t *testing.T) {
	c, s := &liveCreate{}, &startRecorder{}
	m := app.NewModel(app.Deps{State: oneProject(), Terms: map[string]terminal.Terminal{}, Create: c.fn, Start: s.fn})

	newSession(m, "test")

	if len(s.Started) != 1 || s.Started[0] != "new-id" {
		t.Fatalf("started %v, want one terminal for new-id", s.Started)
	}
	if got := m.Selected(); got != "new-id" {
		t.Errorf("Focused() = %q, want the session just created", got)
	}
	if !strings.Contains(m.View().Content, "terminal for test") {
		t.Errorf("the new session's terminal is not rendered:\n%s", m.View().Content)
	}
}

func TestModel_secondSessionAlsoAppears_issue32(t *testing.T) {
	c, s := &liveCreate{}, &startRecorder{}
	m := app.NewModel(app.Deps{State: oneProject(), Terms: map[string]terminal.Terminal{}, Create: c.fn, Start: s.fn})

	newSession(m, "first")
	newSession(m, "second")

	got := m.View().Content
	for _, want := range []string{"first", "second"} {
		if !strings.Contains(got, want) {
			t.Errorf("session %q missing from the sidebar:\n%s", want, got)
		}
	}
}

// If the terminal cannot start, the session must not be added as an
// unfocusable row.
func TestModel_startFailureSurfacesAndAddsNoRow_issue32(t *testing.T) {
	c := &liveCreate{}
	s := &startRecorder{Err: errors.New("pty exhausted")}
	m := app.NewModel(app.Deps{State: oneProject(), Terms: map[string]terminal.Terminal{}, Create: c.fn, Start: s.fn})

	newSession(m, "doomed")

	got := m.View().Content
	if !strings.Contains(got, "pty exhausted") {
		t.Errorf("the start failure is not surfaced:\n%s", got)
	}
	if m.Selected() != "" {
		t.Errorf("Focused() = %q after a failed start, want no selection", m.Selected())
	}
}

// Regression, issue #73: the StartFunc closure froze the pane size at Run
// time, so a session created after a window resize was born at the startup
// size and never resized.
func TestModel_SessionCreatedAfterAResizeIsBornAtTheCurrentPTYSize_issue73(t *testing.T) {
	c, s := &liveCreate{}, &startRecorder{}
	m := app.NewModel(app.Deps{State: oneProject(), Terms: map[string]terminal.Terminal{}, Create: c.fn, Start: s.fn})
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})

	newSession(m, "late")

	if s.W != 171 || s.H != 57 {
		t.Errorf("born at %dx%d, want PTYSize(200,60) = 171x57, not the startup size", s.W, s.H)
	}
}
