package ui_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// Regression, issue #33. The unit tests use an inert Fake and the termwrap
// integration test pumped the command loop by hand, so nothing exercised a
// real terminal *through the model* - which is exactly where the wiring was
// broken. This drives the genuine bubbletea loop the way the program does.
func TestModel_RendersRealProcessOutputThroughTheModel_issue33(t *testing.T) {
	term, err := termwrap.Start(60, 12, exec.Command("printf", "omatty-pumped\\n"))
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	defer func() { _ = term.Close() }()

	st := registry.State{
		Projects: []registry.Project{{Name: "p", Root: "/p"}},
		Sessions: []registry.Session{{ID: "s1", Project: "p", Title: "one"}},
	}
	m := ui.NewModel(ui.Deps{State: st, Terms: map[string]termwrap.Terminal{"s1": term}, Create: noCreate, Start: noStart})

	if got := drive(t, m, "omatty-pumped", 5*time.Second); !strings.Contains(got, "omatty-pumped") {
		t.Errorf("the model never rendered the process output.\ngot:\n%q", got)
	}
}

// drive runs the model's own command loop - Init, then each returned Cmd fed
// back through Update - until want appears or the deadline passes.
func drive(t *testing.T, m *ui.Model, want string, deadline time.Duration) string {
	t.Helper()
	stop := time.Now().Add(deadline)
	pending := []tea.Cmd{m.Init()}
	for time.Now().Before(stop) {
		if strings.Contains(m.View().Content, want) {
			return m.View().Content
		}
		if len(pending) == 0 {
			pending = append(pending, m.Init())
		}
		cmd := pending[0]
		pending = pending[1:]
		if cmd == nil {
			continue
		}
		msgs := make(chan tea.Msg, 1)
		go func(c tea.Cmd) { msgs <- c() }(cmd)
		select {
		case msg := <-msgs:
			if msg == nil {
				continue
			}
			// bubbletea unpacks a batch itself; the harness must too, or the
			// terminal's poll inside Init's batch never runs (issue #71).
			if batch, ok := msg.(tea.BatchMsg); ok {
				pending = append(pending, batch...)
				continue
			}
			if _, next := m.Update(msg); next != nil {
				pending = append(pending, next)
			}
		case <-time.After(300 * time.Millisecond):
		}
	}
	return m.View().Content
}
