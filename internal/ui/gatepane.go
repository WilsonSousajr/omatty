// The gate pane (#231): the review column's fourth mode, showing what the
// project's own gate made of the session's work.
//
// A mode rather than a new pane. Adding a fourth column would have been the
// orchestrator-shaped move - one more surface to watch - where the review
// column already has the layout, the scrolling, the folding and the submit
// path, and a gate result is the same kind of object as a diff: something you
// read, and then act on.

package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/paste"
)

// renderGate draws the column's gate view: one row per step, with the output
// of any step folded open beneath it.
//
// The width is unused: fitBlock cuts every view to the column, and a step's
// command is more useful truncated at the edge than wrapped onto a second row
// that the cursor would then have to account for.
func (m *Model) renderGate(_, h int) []string {
	id := m.review.SessionID
	// A report is real data and outranks everything: it says what the gate
	// actually found, even if the project's gate has since been changed or
	// cleared. Only when there is none do the three "nothing yet" states
	// matter, and they are distinct - "no gate configured" and "configured
	// but not run" call for different actions from the operator.
	report, ran := m.gates[id]
	switch {
	case m.gateRunning[id]:
		return m.pendingGateLines(id)
	case ran && report.Err != nil:
		return []string{"the gate could not run:", "  " + report.Err.Error()}
	case ran:
		return window(m.gateLines(report), m.review.GateOffset, h)
	case len(m.gateFor(id)) > 0:
		return m.pendingGateLines(id)
	}
	return m.noGateLines()
}

// pendingGateLines covers the two states before a verdict exists: a run in
// flight, and a gate that is configured but has not been asked for yet. They
// are separated because showing the last run as if it were current would be a
// verdict about code that has since changed.
func (m *Model) pendingGateLines(id string) []string {
	head := "the gate has not run for this session yet."
	if m.gateRunning[id] {
		head = "running the gate..."
	}
	lines := []string{head, ""}
	steps := m.gateFor(id)
	w := nameWidth(steps)
	for _, step := range steps {
		lines = append(lines, stepRow("  ", "·", step, w, ""))
	}
	return lines
}

// noGateLines explains an absent gate rather than showing an empty pane, which
// would read as "the gate found nothing" - the opposite of the truth.
func (m *Model) noGateLines() []string {
	project := m.projectNameOf(m.review.SessionID)
	return []string{
		"no gate is configured for " + project + ".",
		"",
		"a gate is the project's own check - fmt, vet, lint, tests -",
		"and omatty proposes one by reading the repository:",
		"",
		"    omatty gate " + project,
	}
}

// gateLines is every step, each followed by its output when folded open.
func (m *Model) gateLines(report gate.Report) []string {
	steps := make([]gate.Step, len(report.Results))
	for i, result := range report.Results {
		steps[i] = result.Step
	}
	w := nameWidth(steps)
	var lines []string
	for i, result := range report.Results {
		lines = append(lines, m.gateStepLine(i, result, w))
		if m.review.GateOpen[i] {
			lines = append(lines, indent(result.Output)...)
		}
	}
	return lines
}

// gateStepLine is one step's row: the cursor, its mark, its name and the
// command it ran, so the pane says what was actually executed.
func (m *Model) gateStepLine(i int, result gate.StepResult, nameW int) string {
	cursor := "  "
	if i == m.review.GateCursor {
		cursor = "▸ "
	}
	return stepRow(cursor, verdictMark[result.Verdict], result.Step, nameW, elapsed(result))
}

// stepRow lays out one step for both the pending and the verdict view, so the
// command starts in the same column in each and a row does not shift when its
// verdict replaces the pending mark (#342). took is blank while pending.
func stepRow(cursor, mark string, step gate.Step, nameW int, took string) string {
	return fmt.Sprintf("%s%s %-*s %-6s $ %s", cursor, mark, nameW, step.Name, took, step.Run)
}

// nameWidth is the widest step name, and never less than six, so "fmt" and
// "coverage" in one gate still share a command column (#342).
func nameWidth(steps []gate.Step) int {
	w := 6
	for _, step := range steps {
		w = max(w, len([]rune(step.Name)))
	}
	return w
}

// elapsed is how long a step took, blank for one that never ran.
func elapsed(result gate.StepResult) string {
	if result.Elapsed <= 0 {
		return ""
	}
	return fmt.Sprintf("%.1fs", result.Elapsed.Seconds())
}

// indent sets a step's output apart from the rows, and drops the trailing
// blank a captured stream always ends with.
func indent(out string) []string {
	if out == "" {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		lines = append(lines, "      "+l)
	}
	return lines
}

// projectNameOf is the project a session belongs to, for the line that says
// how to set a gate for it.
func (m *Model) projectNameOf(id string) string {
	for _, sess := range m.state.Sessions {
		if sess.ID == id {
			return sess.Project
		}
	}
	return "<project>"
}

// window is the slice of lines starting at offset that fits in h rows.
func window(lines []string, offset, h int) []string {
	if offset >= len(lines) {
		return nil
	}
	end := min(offset+h, len(lines))
	return lines[offset:end]
}

// onGateKey is the gate view's keymap. It is deliberately the diff's shape -
// j/k to walk, enter to open, esc to leave - so the column's modes do not each
// need learning.
func (m *Model) onGateKey(key string) tea.Cmd {
	switch key {
	case "j", "down":
		m.moveGateCursor(1)
	case "k", "up":
		m.moveGateCursor(-1)
	case "enter":
		m.toggleGateStep()
	case "S", "shift+s", "shift+S":
		// Three spellings, all of which occur: a terminal reporting the
		// modifier sends "shift+s", a legacy one the bare "S", and one that
		// shifts the base key too "shift+S" - the same set modalCommand
		// already handles.
		return m.submitGate()
	case "esc", "ctrl+c":
		// The diff and the tree both hand the keys back rather than close the
		// column; the gate does the same so esc means one thing everywhere.
		m.review.Focused = false
	default:
		// h/l/0, shared by every view (#94). A step's command is the widest
		// thing here and is cut at the column edge, so it has to be reachable.
		m.panKey(key)
	}
	return nil
}

// moveGateCursor walks the steps, stopping at the ends rather than wrapping:
// a gate is a short list read top to bottom.
func (m *Model) moveGateCursor(by int) {
	steps := len(m.gates[m.review.SessionID].Results)
	if steps == 0 {
		return
	}
	m.review.GateCursor = clampTo(m.review.GateCursor+by, 0, steps-1)
	m.review.GateOffset = 0
}

// toggleGateStep folds the step under the cursor open or shut. Output is
// hidden by default because four steps of test output would bury the summary
// the pane exists to show.
func (m *Model) toggleGateStep() {
	if m.review.GateOpen == nil {
		m.review.GateOpen = map[int]bool{}
	}
	m.review.GateOpen[m.review.GateCursor] = !m.review.GateOpen[m.review.GateCursor]
}

// clampTo keeps n within [lo, hi].
func clampTo(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// gateMaxWidth is the widest line the gate view draws, so panning stops at
// the end of the text rather than scrolling into blank space.
func (m *Model) gateMaxWidth() int {
	widest := 0
	for _, line := range m.gateLines(m.gates[m.review.SessionID]) {
		widest = max(widest, lipgloss.Width(line))
	}
	return widest
}

// submitGate sends the gate's failures back into the session that caused
// them, as one bracketed paste (invariant 8).
//
// This is the milestone in one keystroke: from "the gate failed" to "claude
// is fixing it", with the operator still the one who decided it should
// happen. Deliberately not automatic - #233 will run the gate for you, but
// nothing sends a prompt on your behalf.
//
// The same shape as submitReview, down to handing the keys back: after
// sending, the next thing anyone wants is to watch the session work.
func (m *Model) submitGate() tea.Cmd {
	id := m.review.SessionID
	report, ran := m.gates[id]
	if !ran {
		m.lastErr = "no gate has run for this session yet"
		return nil
	}
	body := gate.Compose(report.Results)
	if body == "" {
		m.lastErr = "the gate passed; there is nothing to send"
		return nil
	}
	term := m.terms[id]
	if term == nil {
		m.lastErr = "session " + id + " has no terminal to send to"
		return nil
	}
	m.review.Focused = false
	return term.SendInput(paste.BracketedPaste(body))
}
