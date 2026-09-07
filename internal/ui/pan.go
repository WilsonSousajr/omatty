package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// This file is the review column's horizontal axis. Every content row is drawn
// through fitLine, which cuts at the pane's width; before this, anything past
// that width was unreachable in all three views (issue #94).

// panStep is how far h and l move. One column per press is too slow to read a
// long line with; eight lands roughly an indent level at a time.
const panStep = 8

// ReviewColOffset is how many display cells the review column is scrolled
// right of its content's left edge.
func (m *Model) ReviewColOffset() int { return m.review.ColOffset }

// panReview scrolls the column horizontally by delta cells, clamped to the
// left edge and to the widest line the current view can show. Clamping at the
// far end matters as much as at the near one: without it l walks the text off
// the screen and leaves a blank pane with no clue how far it went.
func (m *Model) panReview(delta int) {
	// A pan that cannot raise the offset needs no ceiling, and skipping the
	// walk for it is what keeps the mouse affordable. ColOffset is only ever
	// set through this function or to 0, so it already sits at or below the
	// ceiling in force and a smaller value cannot exceed it. h was one press
	// per walk; a trackpad flick is dozens of notches in a burst, each one
	// paying reviewMaxWidth on the same goroutine the PTY output queues on -
	// 3 ms per notch at the 256 KiB bound a preview is read to (#125).
	//
	// delta 0 still walks whenever the offset is non-zero: that is onResize
	// asking for exactly this clamp, because a wider window lowers the
	// ceiling under an offset set against a narrower one.
	if delta < 0 || m.review.ColOffset+delta <= 0 {
		m.review.ColOffset = max(m.review.ColOffset+delta, 0)
		return
	}
	w := ReviewWidth(m.width, true) - 2
	last := max(m.reviewMaxWidth()-w, 0)
	m.review.ColOffset = min(m.review.ColOffset+delta, last)
}

// panKey handles the horizontal keys shared by all three views, reporting
// whether it consumed the key. 0 is the way back to the left edge without
// holding h.
func (m *Model) panKey(key string) bool {
	switch key {
	case "h", "left":
		m.panReview(-panStep)
	case "l", "right":
		m.panReview(panStep)
	case "0":
		m.review.ColOffset = 0
	default:
		return false
	}
	return true
}

// widthCache remembers the widest row of one view. It sits on ReviewPane so
// that every reset of the pane drops it, and every site that changes what a
// view draws calls contentChanged (#133).
type widthCache struct {
	view  ReviewView
	width int
	valid bool
}

// contentChanged forgets the memoized width. Called wherever the rows a view
// draws are replaced or retouched; a missed call shows as a pan that stops
// short of a line that grew, which TestPanReview_ABurstWalksTheContentOnce
// pins from the other side.
func (m *Model) contentChanged() { m.review.Widest = widthCache{} }

// reviewMaxWidth is the widest row the current view can draw, measured from
// the same text builders the renderers use so the clamp and the frame can
// never disagree about how wide a row is.
//
// It used to walk the whole content on every press rather than caching, on
// the argument that a cached width would be one more thing to invalidate. That
// budget was set for one l press at human repeat rate (#94); a trackpad flick
// is dozens of notches, and walking a 256 KiB preview per notch blocked
// Update for ~180 ms (#133). The width is memoized per view now, and the
// invalidation lives at the handful of sites that change a view's rows.
func (m *Model) reviewMaxWidth() int {
	if c := m.review.Widest; c.valid && c.view == m.review.View {
		return c.width
	}
	w := m.walkMaxWidth()
	m.review.Widest = widthCache{view: m.review.View, width: w, valid: true}
	return w
}

func (m *Model) walkMaxWidth() int {
	switch m.review.View {
	case ViewTree:
		return m.treeMaxWidth()
	case ViewPreview:
		return m.previewMaxWidth()
	default:
		return m.diffMaxWidth()
	}
}

func (m *Model) previewMaxWidth() int {
	widest := 0
	for i, line := range m.review.Preview.Lines {
		widest = max(widest, lipgloss.Width(previewRow(i, line)))
	}
	return widest
}

func (m *Model) treeMaxWidth() int {
	widest := 0
	for _, n := range m.treeRows() {
		widest = max(widest, lipgloss.Width(treeText(n, m.review.Tree.Collapsed(n.Path))))
	}
	return widest
}

func (m *Model) diffMaxWidth() int {
	comments := m.commentsFor(m.review.SessionID).All()
	widest := 0
	for _, e := range m.review.Entries {
		widest = max(widest, lipgloss.Width(entryText(e, m.review.Diff, comments)))
	}
	return widest
}

// previewRow is one numbered preview line. Tabs are expanded here rather than
// left to the renderer: lipgloss measures a tab as one cell and draws it as
// several, which tears the frame (#21).
func previewRow(i int, line string) string {
	return fmt.Sprintf("%4d  %s", i+1, expandTabs(line))
}

// fitContent draws one content row: panned to the column offset, then cut to
// the pane's width. Only content rows go through it — the title, the error and
// the note editor keep the plain fitLine, because a header that slides
// sideways with the body reads as a broken frame, and the editor is an input
// rather than something to scroll.
func (m *Model) fitContent(text string, w int) string {
	return fitLine(panLine(text, m.review.ColOffset), w)
}

// panMarker names how far the column has scrolled, for the title row. A pane
// whose line numbers have slid out of view should say why rather than just
// appear to have lost them.
func (m *Model) panMarker() string {
	if m.review.ColOffset == 0 {
		return ""
	}
	return fmt.Sprintf(" · +%d", m.review.ColOffset)
}
