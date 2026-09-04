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
	w := ReviewWidth(m.width, true) - 2
	last := max(m.reviewMaxWidth()-w, 0)
	m.review.ColOffset = min(max(m.review.ColOffset+delta, 0), last)
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

// reviewMaxWidth is the widest row the current view can draw, measured from
// the same text builders the renderers use so the clamp and the frame can
// never disagree about how wide a row is.
//
// It walks the whole content on every press rather than caching: a preview is
// bounded to 256 KiB and a diff is smaller, so the walk costs far less than a
// frame, and a cached width would be one more thing to invalidate every time a
// comment rebuilds the entries or a listing is retouched.
func (m *Model) reviewMaxWidth() int {
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
