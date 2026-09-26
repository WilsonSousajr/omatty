// Scrolling the gate view (#421).
//
// The cursor walks steps, not lines: enter folds the step under it, and S
// sends the whole report, so a line cursor would buy nothing. The window is
// what reads through an opened step. j keeps scrolling while the step under
// the cursor runs past the bottom, and only then moves to the next step - a
// pager's shape, so a two-hundred-line failure is read the same way it is in
// any terminal. Before this, GateOffset was only ever set to 0, and the output
// past the first screen, like any step below it, could not be reached.

package ui

import "github.com/WilsonSousajr/omatty/internal/gate"

// gateSpan is one step's lines in gateLines' coordinates: its row, and the last
// line of its output when folded open - its row again when shut.
type gateSpan struct{ start, end int }

// gateSpans is where each step sits in gateLines, counted with the same indent
// gateLines draws with, so the scroll and the frame cannot disagree.
func (m *Model) gateSpans(report gate.Report) []gateSpan {
	spans := make([]gateSpan, len(report.Results))
	line := 0
	for i, result := range report.Results {
		end := line
		if m.review.GateOpen[i] {
			end += len(indent(result.Output))
		}
		spans[i] = gateSpan{start: line, end: end}
		line = end + 1
	}
	return spans
}

// gateRows is how many rows the gate view draws: the whole column body, as it
// has no editor line to give up.
func (m *Model) gateRows() int {
	_, h := PaneSize(m.width, m.height, true)
	return h
}

// gateDown reads on through the step under the cursor while its output runs
// past the bottom of the window, then moves to the next step, stopping at the
// last rather than wrapping.
func (m *Model) gateDown(spans []gateSpan) {
	h, here := m.gateRows(), spans[m.review.GateCursor]
	if here.end >= m.review.GateOffset+h {
		m.review.GateOffset++
		return
	}
	if m.review.GateCursor < len(spans)-1 {
		m.review.GateCursor++
		m.review.GateOffset = ScrollOffset(spans[m.review.GateCursor].start, m.review.GateOffset, h)
	}
}

// gateUp is gateDown backwards: it reads back up to the step's own row before
// leaving it, and arrives at the previous step showing the tail of its output,
// so k retraces exactly what j read.
func (m *Model) gateUp(spans []gateSpan) {
	if spans[m.review.GateCursor].start < m.review.GateOffset {
		m.review.GateOffset--
		return
	}
	if m.review.GateCursor == 0 {
		return
	}
	m.review.GateCursor--
	if prev := spans[m.review.GateCursor]; prev.start < m.review.GateOffset {
		m.review.GateOffset = max(prev.start, prev.end-m.gateRows()+1)
	}
}

// clampGateOffset keeps the window on the content and the cursor's row inside
// it. Folding a long step shut can leave the offset past the end, and a
// cursor that was reading an opened step's output is above the window.
func (m *Model) clampGateOffset() {
	spans := m.gateSpans(m.gates[m.review.SessionID])
	if len(spans) == 0 {
		m.review.GateOffset = 0
		return
	}
	h := m.gateRows()
	off := min(m.review.GateOffset, max(spans[len(spans)-1].end+1-h, 0))
	cursor := spans[min(m.review.GateCursor, len(spans)-1)]
	m.review.GateOffset = ScrollOffset(cursor.start, off, h)
}

// gateWindowOffset is the offset renderGate draws from, clamped to lines. A
// report replaced by a shorter one would otherwise leave the offset past its
// end, and window would return nothing - a blank pane that reads as "the gate
// found nothing".
func gateWindowOffset(offset, lines, h int) int {
	return min(offset, max(lines-h, 0))
}

// gateView is the report's rows in the window, panned like every face's
// (#447: it was the one face whose rows ignored h and l) and with the cursor's
// row in reverse video (#424).
func (m *Model) gateView(report gate.Report, h int) []string {
	plain, styled := m.gateLines(report), m.gateLinesWith(report, m.glyphs.cell)
	off := gateWindowOffset(m.review.GateOffset, len(plain), h)
	cursor, w := -1, m.columnWidth()
	if spans := m.gateSpans(report); m.review.GateCursor < len(spans) {
		cursor = spans[m.review.GateCursor].start
	}
	out := make([]string, 0, h)
	for i := off; i < min(off+h, len(plain)); i++ {
		out = append(out, m.gateRow(plain[i], styled[i], i == cursor, w))
	}
	return out
}

// gateRow is one drawn line of the gate, panned and fitted to the column: its
// verdict coloured (#425), or plain in reverse video on the cursor row, where a
// colour run inside would end the reverse part-way along it.
func (m *Model) gateRow(plain, styled string, cursor bool, w int) string {
	if cursor {
		return cursorStyle.Render(m.fitContent(plain, w))
	}
	return m.fitStyled(styled, w)
}

// pageGate moves the gate by delta presses of j or k, bounded by how many
// presses could possibly change anything: every line once, every step once.
func (m *Model) pageGate(delta int) {
	spans := m.gateSpans(m.gates[m.review.SessionID])
	if len(spans) == 0 {
		return
	}
	step, n := 1, delta
	if delta < 0 {
		step, n = -1, -delta
	}
	for range min(n, spans[len(spans)-1].end+len(spans)) {
		m.moveGateCursor(step)
	}
}

// gatePosition is the cursor's step of how many, or nothing while there is no
// report on show: a run in flight or a failed one has no rows to be among.
func (m *Model) gatePosition() (n, total int) {
	id := m.review.SessionID
	report, ran := m.gates[id]
	if !ran || m.gateRunning[id] || report.Err != nil {
		return 0, 0
	}
	return m.review.GateCursor + 1, len(report.Results)
}

// clickGate puts the cursor on the step drawn at window row winY - the step
// whose row or opened output is there - and reports whether one was.
func (m *Model) clickGate(winY int) bool {
	spans, rows := m.gateSpans(m.gates[m.review.SessionID]), m.gateRows()
	line := winY - reviewTop()
	if len(spans) == 0 || line < 0 || line >= rows {
		return false
	}
	at := gateWindowOffset(m.review.GateOffset, spans[len(spans)-1].end+1, rows) + line
	for i, s := range spans {
		if at >= s.start && at <= s.end {
			m.review.GateCursor = i
			return true
		}
	}
	return false
}
