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
	"github.com/WilsonSousajr/omatty/internal/watcher"
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
	delete(m.gateSent, report.ID) // a fresh report's failures have not been sent
	m.gates[report.ID] = report
	return tea.Batch(m.waitForGate(), m.gateNotice(report), m.loadCoverage(report.ID))
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

// autoGate runs a session's gate when its turn ends, if the operator asked
// for that. The thesis in one behaviour: the agent says it is done, and
// omatty checks.
//
// Off by default and off means off. `go test ./... -race` on every idle costs
// real time and a real fan, so it has to be asked for - and only a gate the
// operator confirmed is ever in state.json to be run (#226).
//
// Guarded on the transition, not the state: the tailer replays, and gating
// again on a repeated TurnEnded would cancel a run in flight to start the
// same one over.
func (m *Model) autoGate(id string, before, after watcher.Status) {
	if !m.gateAuto || before == after || !atRest(after) {
		return
	}
	m.runGate(id)
}

// gateNotice is the desktop notification for a gate that came back red while
// omatty was not being watched - which is exactly when it is worth saying.
//
// The same blurred-window rule M2's notifications follow: focused, you can
// see the card, and a notification would be noise. A green gate is never
// news.
func (m *Model) gateNotice(report gate.Report) tea.Cmd {
	if m.hasFocus || report.Err != nil {
		return nil
	}
	if _, failed := firstNotPassed(report.Results); !failed || len(report.Results) == 0 {
		return nil
	}
	return notifyCmd(m.notifier, "gate failed · "+m.sessionTitle(report.ID))
}
