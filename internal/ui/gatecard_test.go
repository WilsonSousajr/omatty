package ui_test

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func results(names []string, verdicts ...gate.Verdict) []gate.StepResult {
	out := make([]gate.StepResult, len(verdicts))
	for i, v := range verdicts {
		out[i] = gate.StepResult{Step: gate.Step{Name: names[i], Run: names[i]}, Verdict: v}
	}
	return out
}

// M8 fixed the card's height so the renderer and the mouse could share one
// number (#176). The gate line keeps it fixed at three rather than making it
// vary: rowHeight feeds both the window math and the click inverse, and a card
// whose height depended on whether a gate had run would move a click's target
// under it.
func TestCard_isThreeLinesOfTheSidebarWidth_issue230(t *testing.T) {
	m, _ := modelWithFakes(t)
	status(m, "s1", watcher.TurnEnded, time.Now().Add(-4*time.Minute))

	card := m.CardOf("s1")

	if len(card) != 3 {
		t.Fatalf("a card is %d lines, want 3", len(card))
	}
	for i, l := range card {
		if got := lipgloss.Width(l); got != ui.SidebarWidth-1 {
			t.Errorf("line %d is %d cells, want %d", i, got, ui.SidebarWidth-1)
		}
	}
}

// A project with no gate configured shows a blank line, not a claim. This is
// the cost of the fixed height, and it must look like nothing rather than like
// a gate that passed.
func TestCard_noGateRun_leavesTheLineBlank_issue230(t *testing.T) {
	m, _ := modelWithFakes(t)

	// stripSGR leaves the bare rail glyph, so everything past it must be
	// spaces: blank has to look like nothing, not like a gate that passed.
	third := stripSGR(m.CardOf("s1")[2])

	if rest := strings.TrimPrefix(third, stripSGR(ui.Rail())); strings.TrimSpace(rest) != "" {
		t.Errorf("line three = %q, want blank until a gate has run", third)
	}
}

func TestCard_gateStrip_marksEachStepAndNamesTheFailure_issue230(t *testing.T) {
	m, _ := modelWithFakes(t)
	names := []string{"fmt", "vet", "lint", "test"}
	m.SetGateReport("s1", gate.Report{
		ID:      "s1",
		Results: results(names, gate.Pass, gate.Pass, gate.Pass, gate.Fail),
	})

	third := stripSGR(m.CardOf("s1")[2])

	if !strings.Contains(third, "✓✓✓✗") {
		t.Errorf("line three = %q, want one mark per step in order", third)
	}
	if !strings.Contains(third, "test") {
		t.Errorf("line three = %q, want the failing step named", third)
	}
}

// A step after a failure was never checked. Showing it as anything but
// unchecked would claim the gate got further than it did.
func TestCard_stepsAfterAFailure_showAsUnchecked_issue230(t *testing.T) {
	m, _ := modelWithFakes(t)
	names := []string{"fmt", "test", "cov"}
	m.SetGateReport("s1", gate.Report{
		ID:      "s1",
		Results: results(names, gate.Pass, gate.Fail, gate.Pending),
	})

	third := stripSGR(m.CardOf("s1")[2])

	if !strings.Contains(third, "✓✗·") {
		t.Errorf("line three = %q, want the unchecked step shown as ·", third)
	}
}

// A tool that is not installed is not a failing step (invariant 12), and the
// strip must not read as one.
func TestCard_missingTool_readsDifferentlyFromAFailure_issue230(t *testing.T) {
	m, _ := modelWithFakes(t)
	names := []string{"fmt", "lint"}
	m.SetGateReport("s1", gate.Report{ID: "s1", Results: results(names, gate.Pass, gate.Missing)})

	third := stripSGR(m.CardOf("s1")[2])

	if strings.Contains(third, "✗") {
		t.Errorf("line three = %q, want a missing tool not to read as a failure", third)
	}
	if !strings.Contains(third, "?") {
		t.Errorf("line three = %q, want a missing tool marked ?", third)
	}
}

func TestCard_coveragePercentage_reachesTheStrip_issue230(t *testing.T) {
	m, _ := modelWithFakes(t)
	res := results([]string{"test", "cov"}, gate.Pass, gate.Pass)
	res[1].Percent = 88.4
	m.SetGateReport("s1", gate.Report{ID: "s1", Results: res})

	if third := stripSGR(m.CardOf("s1")[2]); !strings.Contains(third, "88.4%") {
		t.Errorf("line three = %q, want the coverage reading", third)
	}
}

// READY is derived, never stored: a green gate, changes to show for it, and a
// session at rest. Any one of the three missing and it is not ready.
func TestCard_ready_needsAGreenGateChangesAndARestingSession_issue230(t *testing.T) {
	green := func(m *ui.Model) {
		m.SetGateReport("s1", gate.Report{ID: "s1", Results: results([]string{"test"}, gate.Pass)})
	}

	t.Run("all three", func(t *testing.T) {
		m, _ := modelWithFakes(t)
		green(m)
		m.SetRepoStat("s1", review.Stat{Branch: "main", Added: 12, Removed: 3})
		status(m, "s1", watcher.TurnEnded, time.Now())
		if third := stripSGR(m.CardOf("s1")[2]); !strings.Contains(third, "READY") {
			t.Errorf("line three = %q, want READY", third)
		}
	})

	t.Run("a green gate with nothing to show for it", func(t *testing.T) {
		m, _ := modelWithFakes(t)
		green(m)
		m.SetRepoStat("s1", review.Stat{Branch: "main"})
		status(m, "s1", watcher.TurnEnded, time.Now())
		if third := stripSGR(m.CardOf("s1")[2]); strings.Contains(third, "READY") {
			t.Errorf("line three = %q, want no READY without a diff", third)
		}
	})

	t.Run("a session still working", func(t *testing.T) {
		m, _ := modelWithFakes(t)
		green(m)
		m.SetRepoStat("s1", review.Stat{Branch: "main", Added: 12})
		status(m, "s1", watcher.ToolStarted, time.Now())
		if third := stripSGR(m.CardOf("s1")[2]); strings.Contains(third, "READY") {
			t.Errorf("line three = %q, want no READY while the session is working", third)
		}
	})
}

// A run that could not happen at all is neither pass nor fail, and the card
// has to say so rather than show an empty strip that reads as "no gate".
func TestCard_aRunThatCouldNotHappen_saysSo_issue230(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.SetGateReport("s1", gate.Report{ID: "s1", Err: errBoom})

	if third := stripSGR(m.CardOf("s1")[2]); !strings.Contains(third, "gate error") {
		t.Errorf("line three = %q, want it to report the failed run", third)
	}
}

// The strip must hold its 27 cells whatever it is given: a long step name, a
// gate of many steps, and a three-digit percentage all at once.
func TestCard_gateStripNeverOverflows_issue230(t *testing.T) {
	m, _ := modelWithFakes(t)
	names := []string{"formatting", "vetting", "linting", "unit-tests", "integration", "coverage"}
	res := results(names, gate.Pass, gate.Pass, gate.Pass, gate.Pass, gate.Fail, gate.Pending)
	res[5].Percent = 100
	m.SetGateReport("s1", gate.Report{ID: "s1", Results: res})

	card := m.CardOf("s1")

	for i, l := range card {
		if got := lipgloss.Width(l); got != ui.SidebarWidth-1 {
			t.Errorf("line %d is %d cells, want %d: %q", i, got, ui.SidebarWidth-1, stripSGR(l))
		}
	}
}

// errBoom stands in for a run that could not start.
var errBoom = errRun("gate: working directory %q is not a directory")

type errRun string

func (e errRun) Error() string { return string(e) }
