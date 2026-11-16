// The gate's own keys past moving (#429): run it again, and search what any
// step printed (#781). Both are display and request only - the verdict is the
// exit code, and nothing here reads output to decide anything (invariant 12).

package app

import (
	"fmt"
	"github.com/WilsonSousajr/omatty/internal/tui/theme"
	"strings"
)

// searchStyle marks a match. Bold and underlined rather than a hue: the
// palette's colours each already mean something (#175), and a match means
// only "here".

// gateSearchKey runs the gate's r, / and n/N.
func (m *Model) gateSearchKey(key string) {
	switch {
	case is(key, gateBind.Rerun):
		m.rerunGate()
	case is(key, gateBind.Search):
		m.review.GateSearch.Active = true
	case is(key, gateBind.NextMatch):
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

// gateHit is one match: a step, and a line of its output as the step draws it
// open. Counted whether the step is open or shut - a search that skipped a
// folded step said a string was absent when it was there (#781).
type gateHit struct{ step, line int }

// setGateSearch applies a query as it is typed, and shows its first match at
// or below the top of the window - or the first of all, when every match is
// above it.
func (m *Model) setGateSearch(query string) {
	m.review.GateSearch.Query = query
	m.contentChanged()
	m.gateHit = -1
	if hits := m.gateHits(); len(hits) > 0 {
		m.showHit(hits, m.firstHitFrom(hits, m.review.GateOffset))
	}
}

// jumpToMatch is n (dir 1) and N (dir -1): the match after, or before, the one
// shown, stopping at the ends as the step cursor does.
func (m *Model) jumpToMatch(dir int) {
	hits := m.gateHits()
	if k := m.gateHit + dir; k >= 0 && k < len(hits) {
		m.showHit(hits, k)
	}
}

// gateHits is every output line, of every step, holding the query, case
// folded, in report order. Step rows are not searched: the query is for what
// a step printed, and a step's name is on screen already.
func (m *Model) gateHits() []gateHit {
	query := strings.ToLower(m.review.GateSearch.Query)
	report, ran := m.gates[m.review.SessionID]
	if query == "" || !ran {
		return nil
	}
	var out []gateHit
	for i, result := range report.Results {
		for j, line := range m.gateOutput(result.Output) {
			if strings.Contains(strings.ToLower(line), query) {
				out = append(out, gateHit{step: i, line: j})
			}
		}
	}
	return out
}

// firstHitFrom is the first hit at or below line top as the face is drawn now,
// a shut step's hits standing at its row; the first of all when none is.
func (m *Model) firstHitFrom(hits []gateHit, top int) int {
	spans := m.gateSpans(m.gates[m.review.SessionID])
	for k, h := range hits {
		at := spans[h.step].start
		if m.review.GateOpen[h.step] {
			at += 1 + h.line
		}
		if at >= top {
			return k
		}
	}
	return 0
}

// showHit opens the step that holds hit k, if it is shut, and shows the line.
func (m *Model) showHit(hits []gateHit, k int) {
	h := hits[k]
	if m.review.GateOpen == nil {
		m.review.GateOpen = map[int]bool{}
	}
	m.gateHit, m.review.GateOpen[h.step] = k, true
	m.contentChanged()
	m.showGateLine(m.gateSpans(m.gates[m.review.SessionID])[h.step].start + 1 + h.line)
}

// searchCount is the title's " [k/N]" after the query, " no match" when
// there is none (#781). Bracketed, as vim's own search count is: a bare k/N
// sat beside the cursor's position mark and read as a second one.
func (m *Model) searchCount() string {
	hits := len(m.gateHits())
	switch {
	case m.review.GateSearch.Query == "":
		return ""
	case hits == 0:
		return " no match"
	case m.gateHit < 0 || m.gateHit >= hits:
		return fmt.Sprintf(" [%d matches]", hits)
	}
	return fmt.Sprintf(" [%d/%d]", m.gateHit+1, hits)
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
		b.WriteString(line[:i] + theme.Search.Render(line[i:i+len(q)]))
		line, lower = line[i+len(q):], lower[i+len(q):]
	}
}
