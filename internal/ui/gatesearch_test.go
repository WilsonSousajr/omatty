package ui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// r runs the gate again from its own face; before #429 the only way was to
// close the column and open it.
func TestGate_RRunsTheGateAgain_issue429(t *testing.T) {
	rec := &recordGateRun{}
	m := modelWithGate(t, rec, fourSteps)
	leader(m, key('g'))
	m.Update(ui.GateMsg(landedReport()))

	press(m, key('r'))

	if len(rec.IDs) != 2 {
		t.Errorf("r asked for %d runs in all, want 2 (open, then r)", len(rec.IDs))
	}
}

// r while a run is in flight asks for nothing and says why.
func TestGate_RWhileRunningSaysSo_issue429(t *testing.T) {
	rec := &recordGateRun{}
	m := modelWithGate(t, rec, fourSteps)
	leader(m, key('g'))

	press(m, key('r'))

	if len(rec.IDs) != 1 || !strings.Contains(m.View().Content, "already running") {
		t.Errorf("r during a run: %d runs asked for, footer %q", len(rec.IDs), frameLines(m)[len(frameLines(m))-1])
	}
}

// An opened step's output wraps to the column: it is prose to read top to
// bottom, and at 23 cells a panned failure line is not read at all.
func TestGate_OutputWrapsToTheColumn_issue429(t *testing.T) {
	rep := landedReport()
	rep.Results[2].Output = "expected the configuration to hold every key and it held none\n"
	m := openGateAndLand(t, 80, rep)

	if body := stripSGR(m.View().Content); !strings.Contains(body, "held none") {
		t.Errorf("the end of a long output line is not on screen without panning:\n%s", body)
	}
}

// needleReport is test's failing output: 200 numbered lines, "needle" on the
// 50th and the 150th, so two windows onto it can be told apart.
func needleReport() gate.Report {
	var b strings.Builder
	for i := range 200 {
		if i == 50 || i == 150 {
			fmt.Fprintf(&b, "needle %d\n", i)
			continue
		}
		fmt.Fprintf(&b, "hay %d\n", i)
	}
	rep := landedReport()
	rep.Results[2].Output = b.String()
	return rep
}

// / searches the opened output: enter keeps the query and shows the first
// match, n and N walk the rest. Display only - the verdict is still the exit
// code (invariant 12).
func TestGate_SlashSearchesTheOpenedOutput_issue429(t *testing.T) {
	m := openGateAndLand(t, 100, needleReport())

	press(m, key('/'))
	typeInto(m, "needle")
	press(m, special(tea.KeyEnter))
	if body := m.View().Content; !strings.Contains(body, ui.SearchHit("needle")+" 50") {
		t.Fatalf("the first match is not on screen, highlighted:\n%s", stripSGR(body))
	}
	if title := columnTitle(m); !strings.Contains(title, "/needle") {
		t.Errorf("the title does not say a search is in force: %q", title)
	}

	press(m, key('n'))
	if body := stripSGR(m.View().Content); !strings.Contains(body, "needle 150") || strings.Contains(body, "needle 50") {
		t.Errorf("n did not move to the second match:\n%s", body)
	}
	press(m, key('N'))
	if body := stripSGR(m.View().Content); !strings.Contains(body, "needle 50") {
		t.Errorf("N did not come back to the first match:\n%s", body)
	}
}

// esc clears a kept search before it leaves the column, as the tree's filter
// does; and a tree filter is never the gate's search.
func TestGate_EscClearsTheSearchFirst_issue429(t *testing.T) {
	m := openGateAndLand(t, 100, needleReport())
	press(m, key('/'))
	typeInto(m, "needle")
	press(m, special(tea.KeyEnter))

	press(m, special(tea.KeyEscape))

	if !m.ReviewFocused() || strings.Contains(m.View().Content, ui.SearchHit("needle")) {
		t.Errorf("esc should clear the search and keep the column's keys")
	}
}

// A paste into the search line searches, as typing does: the paste path asks
// the face which line it owns rather than assuming the tree's.
func TestGate_APasteIntoTheSearchSearches_issue429(t *testing.T) {
	m := openGateAndLand(t, 100, needleReport())
	press(m, key('/'))

	m.Update(tea.PasteMsg{Content: "needle"})
	press(m, special(tea.KeyEnter))

	if body := m.View().Content; !strings.Contains(body, ui.SearchHit("needle")+" 50") {
		t.Errorf("a pasted query did not search:\n%s", stripSGR(body))
	}
}
