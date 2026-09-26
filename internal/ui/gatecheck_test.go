package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// fourSteps is a gate of fmt, vet, test and lint.
var fourSteps = []gate.Step{
	{Name: "fmt", Run: "gofmt -l ."}, {Name: "vet", Run: "go vet ./..."},
	{Name: "test", Run: "go test ./..."}, {Name: "lint", Run: "golangci-lint run"},
}

// landedReport is the report a run of fourSteps returns: fmt passed, vet
// passed, test failed with output, and lint never ran.
func landedReport() gate.Report {
	rs := make([]gate.StepResult, len(fourSteps))
	for i, s := range fourSteps {
		rs[i] = gate.StepResult{Step: s, Verdict: gate.Pass, Elapsed: 1200 * time.Millisecond}
	}
	rs[2].Verdict, rs[2].Output = gate.Fail, "--- FAIL: TestSomething\nexpected 2, got 3\n"
	rs[3].Verdict, rs[3].Elapsed = gate.Pending, 0
	return gate.Report{ID: "s1", Results: rs}
}

// openGateAndLand opens the gate face, which asks for a run, and delivers the
// report the way the Runner does.
func openGateAndLand(t *testing.T, width int, rep gate.Report) *ui.Model {
	t.Helper()
	m := modelWithGate(t, &recordGateRun{}, fourSteps)
	m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
	leader(m, key('g'))
	m.Update(ui.GateMsg(rep))
	return m
}

// The title carries the verdict as counts, so it is visible with the list
// scrolled away - gh pr checks' "1 failing, 2 successful".
func TestGate_TitleSummarisesTheRun_issue428(t *testing.T) {
	m := openGateAndLand(t, 200, landedReport())

	if title := columnTitle(m); !strings.Contains(title, "gate · 1 failing · 2 passed") {
		t.Errorf("the title does not summarise the run: %q", title)
	}
}

// The first failure opens itself when the report lands, with the cursor on
// it: the line the operator came for, without an enter to find it.
func TestGate_TheFirstFailureOpensOnArrival_issue428(t *testing.T) {
	m := openGateAndLand(t, 200, landedReport())

	body := stripSGR(m.View().Content)
	if !strings.Contains(body, "expected 2, got 3") {
		t.Errorf("the failed step's output is not open on arrival:\n%s", body)
	}
	if row := cursorRow(m); !strings.Contains(row, "test") {
		t.Errorf("the cursor is on %q, want the failed step", row)
	}
}

// A passing report opens nothing: there is nothing to read but the list.
func TestGate_APassingRunOpensNothing_issue428(t *testing.T) {
	rep := landedReport()
	rep.Results[2].Verdict, rep.Results[3].Verdict = gate.Pass, gate.Pass
	m := openGateAndLand(t, 200, rep)

	if strings.Contains(stripSGR(m.View().Content), "expected 2, got 3") {
		t.Errorf("a passing run opened a step's output")
	}
}

// Durations are a right-aligned column, and a long one reads in minutes, so
// a slow step and a hung run show at a glance.
func TestGate_DurationsAreARightAlignedColumn_issue428(t *testing.T) {
	rep := landedReport()
	rep.Results[1].Elapsed = 102 * time.Second
	m := openGateAndLand(t, 200, rep)

	body := stripSGR(m.View().Content)
	for _, want := range []string{"fmt      1.2s", "vet     1m42s"} {
		if !strings.Contains(body, want) {
			t.Errorf("no row reads %q:\n%s", want, body)
		}
	}
}

// While a run is in flight the title says so, with the spinner.
func TestGate_ARunInFlightSpinsInTheTitle_issue428(t *testing.T) {
	m := modelWithGate(t, &recordGateRun{}, fourSteps)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	leader(m, key('g'))

	if title := columnTitle(m); !strings.Contains(title, "running 0s") {
		t.Errorf("a run in flight does not say so in the title: %q", title)
	}
}
