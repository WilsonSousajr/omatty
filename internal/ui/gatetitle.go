// The gate face's title and arrival (#428): what the run found, readable with
// the list scrolled away, and the failure open before anyone asks for it -
// GitHub Actions' job page and gh pr checks, in a column.

package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

// gateTitle is "gate · <what the run found> · <session>". The finding comes
// before the name and outranks it: the name is shortened in its middle as the
// tree's is (#285), and dropped when too little of it would be left - the
// sidebar and the pane already name the session, and nothing else on screen
// says the gate's verdict.
func (m *Model) gateTitle(budget int) string {
	head := "gate"
	if found := m.gateSummary(); found != "" {
		head += " · " + found
	}
	room := budget - lipgloss.Width(head+" · ")
	if room < minNameCells {
		return head
	}
	return head + " · " + elideMiddle(m.sessionTitle(m.review.SessionID), room)
}

// gateSummary is the run in flight, with the spinner and how long it has
// been going, or the last report's counts; "" when there is neither.
func (m *Model) gateSummary() string {
	id := m.review.SessionID
	if m.gateRunning[id] {
		took := m.clock().Sub(m.gateStarted[id])
		return fmt.Sprintf("%s running %ds", spinnerFrame(m.clock()), int(took.Seconds()))
	}
	report, ran := m.gates[id]
	if !ran || report.Err != nil {
		return ""
	}
	return verdictCounts(report.Results)
}

// verdictCounts is "1 failing · 6 passed · 1 missing", each part only when
// it is not zero, failures first because they are what the title is for.
// Steps that never ran are not counted: a pending step has no verdict.
func verdictCounts(results []gate.StepResult) string {
	counts := map[gate.Verdict]int{}
	for _, r := range results {
		counts[r.Verdict]++
	}
	var parts []string
	for _, c := range []struct {
		v    gate.Verdict
		word string
	}{{gate.Fail, "failing"}, {gate.Pass, "passed"}, {gate.Missing, "missing"}} {
		if n := counts[c.v]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, c.word))
		}
	}
	return strings.Join(parts, " · ")
}

// openFirstFailure is a report landing on the face: the first failed step
// opened with the cursor on it, the rest shut - the line the operator came
// for, without an enter to find it. A passing report opens nothing.
func (m *Model) openFirstFailure(report gate.Report) {
	m.review.GateOpen = map[int]bool{}
	m.review.GateCursor, m.review.GateOffset = 0, 0
	for i, r := range report.Results {
		if r.Verdict == gate.Fail {
			m.review.GateOpen[i], m.review.GateCursor = true, i
			break
		}
	}
	m.clampGateOffset()
}
