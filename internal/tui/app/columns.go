// The body's columns: the sidebar, the session pane and the review column,
// with a hairline between each pair (#174). Split out of render.go when the
// narrow-window layout (#753) made which columns are drawn a decision of its
// own, and pushed render.go past the length limit.

package app

import "time"

// bodyColumns is every column with the hairline before each one but the
// first, and one header segment per column. The accent hairline and the ink
// title go to the same column: the one keyboardEdge names.
func (m *Model) bodyColumns(termW, termH int, now time.Time) ([]string, []segment) {
	edge := m.keyboardEdge()
	cols, segs := m.sidebarColumn(termH, now)
	if m.zoomed() {
		return m.zoomedColumns(cols, segs, termH)
	}
	cols = append(m.hairlineAfter(cols, edge == edgePane, termH), m.renderTerminal(termW, termH))
	// termW-2: the leading space headerRow adds and one blank before the
	// hairline, so the right-aligned usage never touches it (#177).
	segs = append(segs, segment{title: m.paneSegment(now, termW-2, edge == edgePane), width: termW, owns: edge == edgePane})
	if ReviewWidth(m.width, m.review.Open) == 0 {
		return cols, segs
	}
	rw := m.columnWidth()
	cols = append(cols, hairlineColumn(edge == edgeReview, termH), m.renderReview(rw, termH))
	// rw+gutterCols, because renderReview draws the gutter as well as the
	// content and the header row must span exactly what the body does (#498).
	return cols, append(segs, m.columnSegment(rw+gutterCols, edge == edgeReview))
}

// sidebarColumn is the sidebar and its header segment, or nothing in a window
// too narrow for it (#753).
func (m *Model) sidebarColumn(termH int, now time.Time) ([]string, []segment) {
	if !sidebarShown(m.width) {
		return nil, nil
	}
	return []string{m.renderSidebar(termH, now)}, []segment{{title: m.sidebarSegment(), width: sidebarContentCols}}
}

// hairlineAfter appends the hairline that separates the next column from the
// ones before it; the first column has none, so a window without a sidebar
// starts flush at its left edge (#753).
func (m *Model) hairlineAfter(cols []string, accent bool, rows int) []string {
	if len(cols) == 0 {
		return cols
	}
	return append(cols, hairlineColumn(accent, rows))
}

// zoomedColumns is the body with the column drawn over the session pane: the
// sidebar, the column's own hairline, then the column to the edge (#427).
func (m *Model) zoomedColumns(cols []string, segs []segment, termH int) ([]string, []segment) {
	rw := m.columnWidth()
	cols = append(m.hairlineAfter(cols, true, termH), m.renderReview(rw, termH))
	return cols, append(segs, m.columnSegment(rw+gutterCols, true))
}

// columnSegment is the review column's header and rule segment.
func (m *Model) columnSegment(rw int, owns bool) segment {
	label := m.faceName()
	if m.zoomed() {
		label += " · zoomed"
	}
	return segment{title: m.reviewTitle(rw), width: rw, owns: owns, closable: true, label: label}
}
