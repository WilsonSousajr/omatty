// Running a project's gate from the UI (#231): the request out, the report
// back, and the "in flight" state between them.
//
// The shape is the watcher's, deliberately - a channel of results and a
// func to ask for one - so gate results reach the model exactly the way
// status events already do, and neither needs its own machinery.

package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// GateMsg carries one finished gate run into the model's Update loop.
// Exported so a test can deliver one without a Runner.
type GateMsg gate.Report

// waitForGate blocks on the next gate report and delivers it as a GateMsg. A
// model built without a Runner simply never fires, the way waitForEvent does.
func (m *Model) waitForGate() tea.Cmd {
	if m.gateReports == nil {
		return nil
	}
	return func() tea.Msg { return GateMsg(<-m.gateReports) }
}

// onGate folds a finished run into the model and re-arms the wait.
//
// A report naming a session omatty does not hold is dropped rather than
// stored - the rule the status map already follows for hook events, which can
// name any session id on the machine (#69).
func (m *Model) onGate(msg GateMsg) tea.Cmd {
	report := gate.Report(msg)
	if !m.holds(report.ID) {
		return m.waitForGate()
	}
	delete(m.gateRunning, report.ID)
	m.gates[report.ID] = report
	return m.waitForGate()
}

// holds reports whether a session is one of omatty's own.
func (m *Model) holds(id string) bool {
	for _, sess := range m.state.Sessions {
		if sess.ID == id {
			return true
		}
	}
	return false
}

// runGate asks for a run of the selected session's gate, if its project has
// one and a run is not already going.
//
// Opening the pane runs the gate on purpose. A pane that only showed the last
// result would be a report with no way to ask for a fresh one, and M9 exists
// to shorten the distance between "claude says it is done" and knowing
// whether it is.
func (m *Model) runGate(id string) {
	steps := m.gateFor(id)
	if len(steps) == 0 || m.gateRun == nil || m.gateRunning[id] {
		return
	}
	sess, found := m.sessionByID(id)
	if !found {
		return
	}
	m.gateRunning[id] = true
	m.gateRun(id, sess.Dir, steps)
}

// gateFor is the gate of the project a session belongs to, nil when it has
// none - which is "not configured yet" rather than "configured as nothing"
// (#227).
func (m *Model) gateFor(id string) []gate.Step {
	sess, found := m.sessionByID(id)
	if !found {
		return nil
	}
	for _, p := range m.state.Projects {
		if p.Name == sess.Project {
			return p.Gate
		}
	}
	return nil
}

// sessionByID is the registered session with that id.
func (m *Model) sessionByID(id string) (registry.Session, bool) {
	for _, sess := range m.state.Sessions {
		if sess.ID == id {
			return sess, true
		}
	}
	return registry.Session{}, false
}
