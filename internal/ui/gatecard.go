// The gate strip (#230): a session card's third line, saying what the
// project's own gate made of the work in it.

package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// gateCols is line three's budget: the card's content less the rail, the two
// spaces that indent it and the blank final column, matching line two's sums.
const gateCols = cardCols - 1 - 2 - 1

// cardGate is line three past the rail and its indent: the marks, then what
// they amount to, with any coverage reading right-aligned.
//
//	✓✓✓✗ test       88.4%
//	✓✓✓✓            READY
func (m *Model) cardGate(id string) string {
	report, run := m.gates[id]
	if !run {
		return strings.Repeat(" ", gateCols)
	}
	if report.Err != nil {
		return fitLine("gate error", gateCols)
	}
	right := coverageReading(report.Results)
	left := m.marks(report.Results) + "  " + m.gateLabel(id, report.Results)
	return fitLine(left, gateCols-lipgloss.Width(right)-1) + " " + right
}

// marks is one cell per step, uncoloured as the whole card is.
func (m *Model) marks(results []gate.StepResult) string {
	var b strings.Builder
	for _, r := range results {
		b.WriteString(m.glyphs.mark(verdictState(r.Verdict)))
	}
	return b.String()
}

// gateLabel says what the strip amounts to: the step that stopped the gate, or
// READY when there is nothing left to do to it.
func (m *Model) gateLabel(id string, results []gate.StepResult) string {
	if stopped, found := firstNotPassed(results); found {
		return stopped.Step.Name
	}
	if m.readyToShip(id) {
		return "READY"
	}
	return ""
}

// firstNotPassed is the step the gate stopped on, if it stopped.
func firstNotPassed(results []gate.StepResult) (gate.StepResult, bool) {
	for _, r := range results {
		if r.Verdict != gate.Pass {
			return r, true
		}
	}
	return gate.StepResult{}, false
}

// readyToShip is derived, never stored - state.json holds no status
// (invariant 9), and this is three facts it already has: the gate passed,
// there is something to show for it, and the session is not mid-turn.
//
// All three matter. A green gate over an empty diff is a session that has done
// nothing yet, and a green gate on a session still working is a verdict about
// code that is still moving.
func (m *Model) readyToShip(id string) bool {
	stat, polled := m.repoStat[id]
	if !polled || stat.Added+stat.Removed == 0 {
		return false
	}
	return atRest(m.status[id].Status)
}

// reportedStatus is a session's status with an unreported one read as idle,
// which is what sessionRows already does for the sidebar (#331, #334).
//
// watcher.Status is a string, so its zero value is "" and not StatusIdle - and
// atRest answers false for it. A session nothing has reported on is not
// mid-turn: it is one that has not spoken yet, which is every session at boot
// and every stopped one. Without this, READY never appeared on a stopped
// session with a green gate, and both the ship key and the revert key refused
// such a session with the wrong reason.
//
// Found by running the real binary, not by a test: every fixture in the suite
// reports a status before asserting anything.
func (m *Model) reportedStatus(id string) watcher.Status {
	if s := m.status[id].Status; s != "" {
		return s
	}
	return watcher.StatusIdle
}

// atRest reports whether a session is between turns rather than in one.
func atRest(s watcher.Status) bool {
	return s == watcher.StatusIdle || s == watcher.StatusDone
}

// coverageReading is the percentage a coverage step read, or "" - shown in its
// shortest exact form, because the card has 23 cells for everything.
func coverageReading(results []gate.StepResult) string {
	for _, r := range results {
		if r.Percent > 0 {
			return strconv.FormatFloat(r.Percent, 'f', -1, 64) + "%"
		}
	}
	return ""
}
