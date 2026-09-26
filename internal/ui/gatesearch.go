// The gate's own keys past moving (#429): run it again, and search what an
// opened step printed. Both are display and request only - the verdict is the
// exit code, and nothing here reads output to decide anything (invariant 12).

package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// searchStyle marks a match. Bold and underlined rather than a hue: the
// palette's colours each already mean something (#175), and a match means
// only "here".
var searchStyle = lipgloss.NewStyle().Bold(true).Underline(true)

// gateSearchKey runs the gate's r, / and n/N.
func (m *Model) gateSearchKey(key string) {
	switch key {
	case "r":
		m.rerunGate()
	case "/":
		m.review.GateSearch.Active = true
	case "n":
		m.jumpToMatch(1)
	default:
		m.jumpToMatch(-1)
	}
}

// rerunGate asks for a fresh run from the gate's own face. A run already in
// flight is left alone, and the footer says so rather than nothing happening.
func (m *Model) rerunGate() {
	id := m.review.SessionID
	if m.gateRunning[id] {
		m.lastErr = "the gate is already running"
		return
	}
	m.runGate(id)
}

// leaveGate is esc: a kept search is lifted first, then the keys go back.
func (m *Model) leaveGate() {
	if m.review.GateSearch.Query != "" {
		m.setGateSearch("")
		return
	}
	m.review.Focused = false
}

// setGateSearch applies a query as it is typed, and shows its first match at
// or below the top of the window.
func (m *Model) setGateSearch(query string) {
	m.review.GateSearch.Query = query
	m.contentChanged()
	if at, ok := m.nextMatch(m.review.GateOffset-1, 1); ok {
		m.showGateLine(at)
	}
}

// jumpToMatch is n (dir 1) and N (dir -1): the match after, or before, the one
// at the top of the window.
func (m *Model) jumpToMatch(dir int) {
	if at, ok := m.nextMatch(m.review.GateOffset, dir); ok {
		m.showGateLine(at)
	}
}

// nextMatch is the first match strictly past from in dir.
func (m *Model) nextMatch(from, dir int) (int, bool) {
	matches := m.gateMatches()
	if dir < 0 {
		for i := len(matches) - 1; i >= 0; i-- {
			if matches[i] < from {
				return matches[i], true
			}
		}
		return 0, false
	}
	for _, at := range matches {
		if at > from {
			return at, true
		}
	}
	return 0, false
}

// gateMatches is every opened output line holding the query, case folded, in
// gateLines' coordinates. Step rows are not searched: the query is for what a
// step printed, and a step's name is on screen already.
func (m *Model) gateMatches() []int {
	query := strings.ToLower(m.review.GateSearch.Query)
	report, ran := m.gates[m.review.SessionID]
	if query == "" || !ran {
		return nil
	}
	lines := m.gateLines(report)
	var out []int
	for _, s := range m.gateSpans(report) {
		for i := s.start + 1; i <= s.end; i++ {
			if strings.Contains(strings.ToLower(lines[i]), query) {
				out = append(out, i)
			}
		}
	}
	return out
}

// showGateLine puts line at the top of the window and the cursor on the step
// that printed it: the pager's shape, where the cursor's row may be above.
func (m *Model) showGateLine(line int) {
	report := m.gates[m.review.SessionID]
	for i, s := range m.gateSpans(report) {
		if line >= s.start && line <= s.end {
			m.review.GateCursor = i
		}
	}
	m.review.GateOffset = gateWindowOffset(line, len(m.gateLines(report)), m.gateRows())
}

// highlightMatches draws every case-folded occurrence of query in line with
// searchStyle.
func highlightMatches(line, query string) string {
	if query == "" {
		return line
	}
	lower, q := strings.ToLower(line), strings.ToLower(query)
	var b strings.Builder
	for {
		i := strings.Index(lower, q)
		if i < 0 {
			return b.String() + line
		}
		b.WriteString(line[:i] + searchStyle.Render(line[i:i+len(q)]))
		line, lower = line[i+len(q):], lower[i+len(q):]
	}
}
