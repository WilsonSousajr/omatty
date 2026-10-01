package ui

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// A restart that lands after its session was archived has no pane to go to.
// Kept, its terminal would run a claude nobody can see or stop; it is closed
// instead (#653).
func TestModel_aRestartLandingAfterArchiveClosesItsTerminal_issue653(t *testing.T) {
	m := NewModel(Deps{State: sessions.State{}, Terms: map[string]termwrap.Terminal{}})
	fake := termwrap.NewFake("")

	m.onSessionStarted(sessionStartedMsg{sess: sessions.Session{ID: "gone"}, term: fake, restart: true})

	if !fake.Closed {
		t.Error("the terminal of a session archived mid-restart was left running")
	}
	if _, kept := m.terms["gone"]; kept {
		t.Error("an archived session got a terminal back")
	}
}
