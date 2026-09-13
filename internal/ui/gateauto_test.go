package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// autoModel is a session whose project has a gate, with the run recorded, and
// auto-run set either way.
func autoModel(t *testing.T, rec *recordGateRun, auto bool) (*ui.Model, *notify.Fake) {
	t.Helper()
	st := registry.State{
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty",
			Gate: []gate.Step{{Name: "test", Run: "go test ./..."}}}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "one", Dir: "/p/omatty"}},
	}
	deps := baseDeps(st, fakeTermsFor(st))
	fake := &notify.Fake{}
	deps.GateRun, deps.GateAuto, deps.Notifier = rec.Run, auto, fake
	m := ui.NewModel(deps)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m, fake
}

// The thesis in one behaviour: the agent says it is done, and omatty checks.
func TestModel_autoRun_gatesASessionThatGoesIdle_issue233(t *testing.T) {
	rec := &recordGateRun{}
	m, _ := autoModel(t, rec, true)

	status(m, "s1", watcher.TurnEnded, time.Now())

	if len(rec.IDs) != 1 || rec.IDs[0] != "s1" {
		t.Fatalf("auto-run gated %v, want [s1] when the turn ended", rec.IDs)
	}
}

// Off by default, and off means off. `go test ./... -race` on every idle is
// expensive enough that it has to be asked for.
func TestModel_autoRunIsOffUnlessAskedFor_issue233(t *testing.T) {
	rec := &recordGateRun{}
	m, _ := autoModel(t, rec, false)

	status(m, "s1", watcher.TurnEnded, time.Now())

	if len(rec.IDs) != 0 {
		t.Errorf("auto-run started %d gates while off, want none", len(rec.IDs))
	}
}

// A session mid-turn has nothing worth gating: the code is still moving.
func TestModel_autoRun_ignoresATurnStillRunning_issue233(t *testing.T) {
	rec := &recordGateRun{}
	m, _ := autoModel(t, rec, true)

	status(m, "s1", watcher.ToolStarted, time.Now())

	if len(rec.IDs) != 0 {
		t.Errorf("auto-run gated a working session %v, want none", rec.IDs)
	}
}

// Two events that leave the session in the same state must not gate twice -
// the tailer replays, and a second run would cancel the first for nothing.
func TestModel_autoRun_doesNotRestartOnARepeatedStatus_issue233(t *testing.T) {
	rec := &recordGateRun{}
	m, _ := autoModel(t, rec, true)

	status(m, "s1", watcher.TurnEnded, time.Now())
	status(m, "s1", watcher.TurnEnded, time.Now().Add(time.Second))

	if len(rec.IDs) != 1 {
		t.Errorf("auto-run started %d gates for one transition, want 1", len(rec.IDs))
	}
}

// A project with no gate has nothing to run, however idle its sessions go.
func TestModel_autoRun_withNoGateConfigured_runsNothing_issue233(t *testing.T) {
	rec := &recordGateRun{}
	st := registry.State{
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "one", Dir: "/p/omatty"}},
	}
	deps := baseDeps(st, fakeTermsFor(st))
	deps.GateRun, deps.GateAuto = rec.Run, true
	m := ui.NewModel(deps)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	status(m, "s1", watcher.TurnEnded, time.Now())

	if len(rec.IDs) != 0 {
		t.Errorf("auto-run started %d gates for a project with none", len(rec.IDs))
	}
}

// A red gate on a session you are not watching is exactly what a notification
// is for - the same blurred-window rule M2's notifications already follow.
func TestModel_aRedGateNotifiesWhenOmattyIsBlurred_issue233(t *testing.T) {
	rec := &recordGateRun{}
	m, fake := autoModel(t, rec, true)
	m.Update(tea.FocusMsg{})
	m.Update(tea.BlurMsg{})

	_, cmd := m.Update(ui.GateMsg(gate.Report{ID: "s1", Results: []gate.StepResult{
		{Step: gate.Step{Name: "test"}, Verdict: gate.Fail, ExitCode: 1},
	}}))
	runCmd(cmd)

	if len(fake.Sent) == 0 {
		t.Fatal("a red gate posted no notification while blurred")
	}
	if body := fake.Sent[0].Body; !strings.Contains(body, "gate") {
		t.Errorf("notification = %q, want it to name the gate", body)
	}
}

// Focused, you can see it. A notification would be noise.
func TestModel_aRedGateIsSilentWhileOmattyIsFocused_issue233(t *testing.T) {
	rec := &recordGateRun{}
	m, fake := autoModel(t, rec, true)
	m.Update(tea.FocusMsg{})

	_, cmd := m.Update(ui.GateMsg(gate.Report{ID: "s1", Results: []gate.StepResult{
		{Step: gate.Step{Name: "test"}, Verdict: gate.Fail},
	}}))
	runCmd(cmd)

	if len(fake.Sent) != 0 {
		t.Errorf("a red gate notified while focused: %v", fake.Sent)
	}
}

// A green gate is not news.
func TestModel_aGreenGateNeverNotifies_issue233(t *testing.T) {
	rec := &recordGateRun{}
	m, fake := autoModel(t, rec, true)
	m.Update(tea.FocusMsg{})
	m.Update(tea.BlurMsg{})

	_, cmd := m.Update(ui.GateMsg(gate.Report{ID: "s1", Results: []gate.StepResult{
		{Step: gate.Step{Name: "test"}, Verdict: gate.Pass},
	}}))
	runCmd(cmd)

	if len(fake.Sent) != 0 {
		t.Errorf("a passing gate notified: %v", fake.Sent)
	}
}
