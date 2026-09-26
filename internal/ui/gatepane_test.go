package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func gateReport(verdicts ...gate.Verdict) gate.Report {
	names := []string{"fmt", "vet", "test", "cov"}
	out := make([]gate.StepResult, len(verdicts))
	for i, v := range verdicts {
		out[i] = gate.StepResult{
			Step:    gate.Step{Name: names[i], Run: names[i] + " ./..."},
			Verdict: v,
		}
	}
	return gate.Report{ID: "s1", Results: out}
}

// The gate is a third mode of the review column rather than a fourth pane.
// Adding a pane would be the orchestrator-shaped move; the column already has
// the layout, the scrolling and the submit path, and a gate is the same kind
// of object as a diff - something you read, then act on (#231).
func TestModel_LeaderGOpensTheGateInTheReviewColumn_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gateReport(gate.Pass, gate.Pass, gate.Fail, gate.Pending))

	leader(m, key('g'))

	if !m.ReviewOpen() || !m.ReviewFocused() {
		t.Fatalf("after ctrl+o g: open=%v focused=%v, want both true", m.ReviewOpen(), m.ReviewFocused())
	}
	body := m.View().Content
	for _, want := range []string{"fmt", "vet", "test", "cov"} {
		if !strings.Contains(body, want) {
			t.Errorf("the gate pane does not list %q:\n%s", want, body)
		}
	}
}

func TestModel_LeaderGAgainClosesTheColumn_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gateReport(gate.Pass))
	leader(m, key('g'))

	leader(m, key('g'))

	if m.ReviewOpen() {
		t.Error("ctrl+o g twice left the column open")
	}
}

// Switching between the column's modes must not close it: d then g is a
// change of view, the same way d then f already is.
func TestModel_GateAndDiffSwitchWithoutClosing_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gateReport(gate.Pass))

	leader(m, key('d'))
	leader(m, key('g'))

	if !m.ReviewOpen() {
		t.Fatal("switching from the diff to the gate closed the column")
	}
	if !strings.Contains(m.View().Content, "gate") {
		t.Errorf("the column is not showing the gate:\n%s", m.View().Content)
	}
}

// The title says which view has the keys, as the other three do.
func TestModel_gatePaneTitleNamesTheView_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gateReport(gate.Pass, gate.Fail))

	leader(m, key('g'))

	if head := strings.SplitN(m.View().Content, "\n", 2)[0]; !strings.Contains(head, "gate") {
		t.Errorf("header = %q, want it to name the gate view", head)
	}
}

// A session whose project has no gate must say so. An empty pane would read
// as "the gate found nothing", which is the opposite of the truth.
func TestModel_gatePaneWithNoGateConfigured_saysSo_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)

	leader(m, key('g'))

	body := m.View().Content
	if !strings.Contains(body, "no gate") {
		t.Errorf("the pane does not explain the absent gate:\n%s", body)
	}
	if !strings.Contains(body, "omatty gate") {
		t.Errorf("the pane does not say how to set one:\n%s", body)
	}
}

// A step's output is folded away until asked for: four steps of test output
// would bury the summary the pane exists to show.
func TestModel_gatePaneFoldsAStepOpen_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	rep := gateReport(gate.Pass, gate.Fail)
	rep.Results[1].Output = "--- FAIL: TestThing\n    thing_test.go:12: got 1, want 2\n"
	m.SetGateReport("s1", rep)
	leader(m, key('g'))

	if strings.Contains(m.View().Content, "thing_test.go") {
		t.Fatal("a step's output is shown before it is folded open")
	}
	press(m, key('j'))
	press(m, special(13)) // enter

	if !strings.Contains(m.View().Content, "thing_test.go") {
		t.Errorf("enter did not fold the step open:\n%s", m.View().Content)
	}
}

// esc leaves the column, the same way it does from the diff and the tree.
func TestModel_escLeavesTheGatePane_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gateReport(gate.Pass))
	leader(m, key('g'))

	press(m, special(27)) // esc

	if m.ReviewFocused() {
		t.Error("esc did not hand the keys back")
	}
}

// A run that could not happen is neither pass nor fail, and the pane has to
// carry the reason rather than show an empty list.
func TestModel_gatePaneShowsAFailedRun_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gate.Report{ID: "s1", Err: errRun("working directory is gone")})

	leader(m, key('g'))

	if !strings.Contains(m.View().Content, "working directory is gone") {
		t.Errorf("the pane does not carry the run's error:\n%s", m.View().Content)
	}
}

// The help modal lists every leader key; a binding missing from it is a
// binding nobody finds (#103).
func TestHelp_listsTheGateBinding_issue231(t *testing.T) {
	m, _ := modelWithFakes(t)
	leader(m, key('?'))

	if body := m.View().Content; !strings.Contains(body, "gate") {
		t.Errorf("the help modal does not list the gate binding:\n%s", body)
	}
	_ = ui.SidebarWidth
}

// The cursor stops at the ends rather than wrapping: a gate is a short list
// read top to bottom, and wrapping from the last step to the first would
// suggest an order that is not there.
func TestModel_gateCursorStopsAtTheEnds_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gateReport(gate.Pass, gate.Fail))
	leader(m, key('g'))

	for range 5 {
		press(m, key('k')) // up, from the top
	}
	if !strings.Contains(m.View().Content, "▸ ✓ fmt") {
		t.Errorf("cursor left the first step going up:\n%s", m.View().Content)
	}

	for range 5 {
		press(m, key('j')) // down, past the end
	}
	if !strings.Contains(m.View().Content, "▸ ✗ vet") {
		t.Errorf("cursor left the last step going down:\n%s", m.View().Content)
	}
}

// A pane with nothing in it must not move a cursor that has nowhere to go.
func TestModel_gateCursorWithNoSteps_doesNothing_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gate.Report{ID: "s1"})
	leader(m, key('g'))

	press(m, key('j'))
	press(m, key('k'))

	if m.View().Content == "" {
		t.Error("the pane stopped rendering after moving a cursor over no steps")
	}
}

// Panning has to know how wide the gate's text is, or l would scroll into
// blank space past the end of the longest command.
func TestModel_gatePanStopsAtTheWidestLine_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	rep := gateReport(gate.Pass, gate.Fail)
	rep.Results[1].Step.Run = strings.Repeat("x", 200)
	m.SetGateReport("s1", rep)
	leader(m, key('g'))

	for range 40 {
		press(m, key('l'))
	}
	afterPanning := m.View().Content

	press(m, key('0')) // back to the left edge
	if m.View().Content == afterPanning {
		t.Error("panning right then home changed nothing; the gate view is not pannable")
	}
	if !strings.Contains(m.View().Content, "fmt") {
		t.Errorf("home did not return to the left edge:\n%s", m.View().Content)
	}
}

// A step with no output folds open to nothing rather than to a blank row that
// looks like output the pane failed to show.
func TestModel_foldingAStepWithNoOutput_addsNothing_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gateReport(gate.Pass, gate.Pass))
	leader(m, key('g'))
	before := strings.Count(m.View().Content, "\n")

	press(m, special(13)) // enter on a step whose Output is ""

	if after := strings.Count(m.View().Content, "\n"); after != before {
		t.Errorf("folding an empty step changed the pane height: %d -> %d", before, after)
	}
}

// Elapsed is shown for a step that ran and omitted for one that did not, so a
// Pending row does not claim to have taken 0.0s.
func TestModel_gatePaneShowsElapsedOnlyForStepsThatRan_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	rep := gateReport(gate.Pass, gate.Pending)
	rep.Results[0].Elapsed = 1500 * time.Millisecond
	m.SetGateReport("s1", rep)

	leader(m, key('g'))

	body := m.View().Content
	if !strings.Contains(body, "1.5s") {
		t.Errorf("the finished step does not show its time:\n%s", body)
	}
	if strings.Contains(body, "0.0s") {
		t.Errorf("a step that never ran claims a duration:\n%s", body)
	}
}

// A long gate scrolls rather than overflowing the column.
func TestModel_gatePaneScrollsPastTheColumnHeight_issue231(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	rep := gateReport(gate.Pass, gate.Fail)
	rep.Results[1].Output = strings.Repeat("a line of failure output\n", 200)
	m.SetGateReport("s1", rep)
	leader(m, key('g'))

	press(m, key('j'))
	press(m, special(13)) // fold the flooding step open

	// The view is bounded by the column, not by the output.
	if lines := strings.Count(m.View().Content, "\n"); lines > 60 {
		t.Errorf("the pane drew %d lines; the column must bound it", lines)
	}
}

// dollarColumns is the display column of each step's "$ <run>" in body, keyed
// by the command, so a test can compare where the command starts across rows.
func dollarColumns(t *testing.T, body string, runs ...string) map[string]int {
	t.Helper()
	cols := map[string]int{}
	for _, line := range strings.Split(ansi.Strip(body), "\n") {
		for _, run := range runs {
			if i := strings.Index(line, "$ "+run); i >= 0 {
				cols[run] = ansi.StringWidth(line[:i])
			}
		}
	}
	for _, run := range runs {
		if _, ok := cols[run]; !ok {
			t.Fatalf("no row shows $ %s:\n%s", run, body)
		}
	}
	return cols
}

// The command column must not move: rows with names of different lengths
// line up, and a row does not shift when its verdict replaces the pending
// mark. Before #342 a pending row put four spaces after the name, so "test"
// sat one column right of "fmt", and a name longer than six characters broke
// the verdict view's fixed width as well.
func TestModel_gateRowsKeepOneCommandColumn_issue342(t *testing.T) {
	steps := []gate.Step{
		{Name: "fmt", Run: "gofmt -l ."},
		{Name: "test", Run: "go test ./..."},
		{Name: "coverage", Run: "./cov.sh"},
	}
	runs := []string{"gofmt -l .", "go test ./...", "./cov.sh"}
	st := registry.State{
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty", Gate: steps}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "one", Dir: "/p/omatty"}},
	}
	m := ui.NewModel(baseDeps(st, fakeTermsFor(st))) // no runner: the pending view
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	leader(m, key('g'))

	pending := dollarColumns(t, m.View().Content, runs...)

	results := make([]gate.StepResult, len(steps))
	for i, step := range steps {
		results[i] = gate.StepResult{Step: step, Verdict: gate.Pass, Elapsed: 1500 * time.Millisecond}
	}
	m.SetGateReport("s1", gate.Report{ID: "s1", Results: results})
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30}) // drop the memoised pending frame
	verdict := dollarColumns(t, m.View().Content, runs...)

	want := pending[runs[0]]
	for _, run := range runs {
		if pending[run] != want {
			t.Errorf("pending: $ %s at column %d, want %d (the column of $ %s)", run, pending[run], want, runs[0])
		}
		if verdict[run] != want {
			t.Errorf("verdict: $ %s at column %d, want %d, where it sat while pending", run, verdict[run], want)
		}
	}
}

// Regression, issue #421: the gate could not scroll. moveGateCursor set
// GateOffset to 0 on every move and nothing else wrote it, so an opened step's
// output was readable only as far as the first screen - j went straight to the
// next step. j now reads down through an opened step before moving on.
func TestModel_gatePaneReachesTheEndOfAnOpenedStep_issue421(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	rep := gateReport(gate.Fail, gate.Pass)
	rep.Results[0].Output = numberedLines(200)
	m.SetGateReport("s1", rep)
	leader(m, key('g'))
	press(m, special(13)) // open fmt, the step under the cursor

	if !pressUntil(m, key('j'), 300, "line 199") {
		t.Fatalf("j never reached the opened step's last line:\n%s", m.View().Content)
	}
	if !pressUntil(m, key('j'), 3, "▸ ✓ vet") {
		t.Errorf("past the output, j did not move on to the next step:\n%s", m.View().Content)
	}
	if !pressUntil(m, key('k'), 300, "▸ ✗ fmt") {
		t.Errorf("k never read back up to the opened step's row:\n%s", m.View().Content)
	}
}

// Regression, issue #421: a gate with more steps than the column has rows
// walked its cursor off the bottom of the window, where nobody could see it.
func TestModel_gateCursorStaysVisibleInALongGate_issue421(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	results := make([]gate.StepResult, 40)
	for i := range results {
		name := fmt.Sprintf("s%02d", i)
		results[i] = gate.StepResult{Step: gate.Step{Name: name, Run: "true"}, Verdict: gate.Pass}
	}
	m.SetGateReport("s1", gate.Report{ID: "s1", Results: results})
	leader(m, key('g'))

	for range 39 {
		press(m, key('j'))
	}
	if body := m.View().Content; !strings.Contains(body, "▸ ✓ s39") {
		t.Errorf("the cursor on the last step is not on screen:\n%s", body)
	}
	for range 39 {
		press(m, key('k'))
	}
	if body := m.View().Content; !strings.Contains(body, "▸ ✓ s00") {
		t.Errorf("k did not bring the first step back:\n%s", body)
	}
}

// numberedLines is n lines of step output, "line 0" to "line n-1", so a test
// can ask for one by number.
func numberedLines(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

// pressUntil presses k up to limit times and reports whether want appeared in
// the frame - before the first press or after any of them.
func pressUntil(m *ui.Model, k tea.KeyPressMsg, limit int, want string) bool {
	for range limit {
		if strings.Contains(m.View().Content, want) {
			return true
		}
		press(m, k)
	}
	return strings.Contains(m.View().Content, want)
}

// Folding a step shut while reading deep in its output must not leave the
// window past the end of what is left: the pane shows the step's row, with the
// cursor on it, rather than blank space (#421).
func TestModel_foldingShutFromDeepInTheOutputShowsTheStep_issue421(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	rep := gateReport(gate.Fail, gate.Pass)
	rep.Results[0].Output = numberedLines(200)
	m.SetGateReport("s1", rep)
	leader(m, key('g'))
	press(m, special(13))
	pressUntil(m, key('j'), 300, "line 199")

	press(m, special(13)) // fold it shut again

	if body := m.View().Content; !strings.Contains(body, "▸ ✗ fmt") || !strings.Contains(body, "✓ vet") {
		t.Errorf("after folding shut, the pane should show both steps with the cursor on fmt:\n%s", body)
	}
}

// A new report shorter than where the window stood is drawn from its top, not
// from past its end, which would be a blank pane saying nothing (#421).
func TestModel_aShorterReportIsNotDrawnBlank_issue421(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	rep := gateReport(gate.Fail, gate.Pass)
	rep.Results[0].Output = numberedLines(200)
	m.SetGateReport("s1", rep)
	leader(m, key('g'))
	press(m, special(13))
	pressUntil(m, key('j'), 300, "line 199")

	m.SetGateReport("s1", gateReport(gate.Pass, gate.Pass))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24}) // drop the memoised frame

	if body := m.View().Content; !strings.Contains(body, "✓ fmt") {
		t.Errorf("the shorter report was not drawn:\n%s", body)
	}
}
