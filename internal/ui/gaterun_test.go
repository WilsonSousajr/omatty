package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// recordGateRun is the named fake for the run request, so a failure message
// says which session was gated in which directory.
type recordGateRun struct {
	IDs   []string
	Dirs  []string
	Steps [][]gate.Step
}

func (r *recordGateRun) Run(id, dir string, steps []gate.Step) {
	r.IDs = append(r.IDs, id)
	r.Dirs = append(r.Dirs, dir)
	r.Steps = append(r.Steps, steps)
}

// Opening the pane runs the gate. A pane that only showed the last result
// would be a report with no way to ask for a new one, and the whole point of
// M9 is shortening the loop from "claude says done" to "I know whether it is".
func TestModel_LeaderGRunsTheProjectsGate_issue231(t *testing.T) {
	rec := &recordGateRun{}
	m := modelWithGate(t, rec, []gate.Step{{Name: "test", Run: "go test ./..."}})

	leader(m, key('g'))

	if len(rec.IDs) != 1 || rec.IDs[0] != "s1" {
		t.Fatalf("gate run for %v, want [s1]", rec.IDs)
	}
	if rec.Dirs[0] != "/p/omatty" {
		t.Errorf("gate ran in %q, want the session's own directory", rec.Dirs[0])
	}
	if len(rec.Steps[0]) != 1 || rec.Steps[0][0].Run != "go test ./..." {
		t.Errorf("gate ran %+v, want the project's configured steps", rec.Steps[0])
	}
}

// A project with no gate must not start a run, and must say what to do about
// it rather than showing an empty pane.
func TestModel_LeaderGWithNoGateConfigured_runsNothingAndSaysSo_issue231(t *testing.T) {
	rec := &recordGateRun{}
	m := modelWithGate(t, rec, nil)

	leader(m, key('g'))

	if len(rec.IDs) != 0 {
		t.Errorf("a project with no gate started %d runs, want none", len(rec.IDs))
	}
	body := m.View().Content
	if !strings.Contains(body, "no gate is configured") {
		t.Errorf("the pane does not say the gate is unset:\n%s", body)
	}
	if !strings.Contains(body, "omatty gate omatty") {
		t.Errorf("the pane does not say how to set one:\n%s", body)
	}
}

// While a run is in flight the pane says so. Pressing g and seeing the last
// run's verdict would be a lie about code that has since changed.
func TestModel_gatePaneSaysWhenARunIsInFlight_issue231(t *testing.T) {
	m := modelWithGate(t, &recordGateRun{}, []gate.Step{{Name: "test", Run: "true"}})

	leader(m, key('g'))

	if body := m.View().Content; !strings.Contains(body, "running") {
		t.Errorf("the pane does not show the run in flight:\n%s", body)
	}
}

// The report lands through the same shape the watcher's events use, and
// replaces whatever the pane was showing.
func TestModel_aGateReportReachesThePane_issue231(t *testing.T) {
	m := modelWithGate(t, &recordGateRun{}, []gate.Step{{Name: "test", Run: "go test ./..."}})
	leader(m, key('g'))

	m.Update(ui.GateMsg(gate.Report{
		ID: "s1",
		Results: []gate.StepResult{{
			Step:    gate.Step{Name: "test", Run: "go test ./..."},
			Verdict: gate.Fail,
			Output:  "--- FAIL: TestThing\n",
		}},
	}))

	body := m.View().Content
	if strings.Contains(body, "running") {
		t.Errorf("the pane still says running after the report landed:\n%s", body)
	}
	if !strings.Contains(body, "test") {
		t.Errorf("the report did not reach the pane:\n%s", body)
	}
}

// A report for a session omatty does not hold is ignored rather than stored:
// the same rule the status map follows for hook events (#69).
func TestModel_aGateReportForAnUnknownSession_isIgnored_issue231(t *testing.T) {
	m := modelWithGate(t, &recordGateRun{}, []gate.Step{{Name: "test", Run: "true"}})

	m.Update(ui.GateMsg(gate.Report{ID: "not-a-session", Results: []gate.StepResult{{Verdict: gate.Pass}}}))

	if m.GateReportCount() != 0 {
		t.Errorf("a report for an unregistered session was stored")
	}
}

// modelWithGate is a one-project, one-session model whose project carries the
// given gate, with the run request recorded rather than executed.
func modelWithGate(t *testing.T, rec *recordGateRun, steps []gate.Step) *ui.Model {
	t.Helper()
	st := registry.State{
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty", Gate: steps}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "one", Dir: "/p/omatty", Branch: "main"}},
	}
	deps := baseDeps(st, fakeTermsFor(st))
	deps.GateRun = rec.Run
	m := ui.NewModel(deps)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}
