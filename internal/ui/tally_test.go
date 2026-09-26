package ui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// tallyRecorder is a named TallyFunc fake recording each run it was told about.
type tallyRecorder struct {
	Runs   []string
	Passed []bool
	Err    error
}

func (r *tallyRecorder) fn(project string, passed bool) error {
	r.Runs, r.Passed = append(r.Runs, project), append(r.Passed, passed)
	return r.Err
}

func modelWithTally(t *testing.T, rec *tallyRecorder, auto bool) *ui.Model {
	t.Helper()
	st := registry.State{
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty", Gate: []gate.Step{{Name: "test", Run: "go test ./..."}}}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "one", Dir: "/p/omatty"}},
	}
	d := baseDeps(st, fakeTermsFor(st))
	d.Tally, d.GateAuto = rec.fn, auto
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

// #332's rate is over "the gate runs that followed a turn", which is the
// auto-gate path and not a run the operator asked for by hand.
func TestModel_aTurnFollowedByAGreenGateCountsAsFirstPass_issue332(t *testing.T) {
	rec := &tallyRecorder{}
	m := modelWithTally(t, rec, true)

	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())
	deliver(m, second(m.Update(passingReport())))

	if len(rec.Runs) != 1 || rec.Runs[0] != "omatty" {
		t.Fatalf("tallied %v, want one run for omatty", rec.Runs)
	}
	if !rec.Passed[0] {
		t.Error("a passing report was tallied as a failure")
	}
}

// A failing run still counts as a run: the rate is a share, and leaving the
// failures out would make every project look perfect.
func TestModel_aFailingGateStillCountsAsARun_issue332(t *testing.T) {
	rec := &tallyRecorder{}
	m := modelWithTally(t, rec, true)

	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())
	deliver(m, second(m.Update(ui.GateMsg(failingReport()))))

	if len(rec.Passed) != 1 {
		t.Fatalf("tallied %v, want one run", rec.Passed)
	}
	if rec.Passed[0] {
		t.Error("a failing report was tallied as a pass")
	}
}

// A run the operator started by hand is not "a gate run that followed a turn",
// so it is not in the rate. Counting it would measure how often somebody
// re-ran a gate they already knew was red.
func TestModel_aGateRunByHandIsNotTallied_issue332(t *testing.T) {
	rec := &tallyRecorder{}
	m := modelWithTally(t, rec, false)

	leader(m, key('g')) // ctrl+o g runs it by hand
	deliver(m, second(m.Update(passingReport())))

	if len(rec.Runs) != 0 {
		t.Errorf("tallied %v, want a hand-run gate left out of the rate", rec.Runs)
	}
}

// Measuring must never be able to break a session. A store that will not save
// is logged, not surfaced, and the gate report still lands.
func TestModel_aTallyThatWillNotSaveDoesNotDisturbTheReport_issue332(t *testing.T) {
	rec := &tallyRecorder{Err: errListing}
	m := modelWithTally(t, rec, true)

	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())
	deliver(m, second(m.Update(passingReport())))

	if got := m.View().Content; got == "" {
		t.Error("the frame did not render after a failed measurement")
	}
	if len(rec.Runs) != 1 {
		t.Errorf("tallied %v, want the run still attempted", rec.Runs)
	}
}
