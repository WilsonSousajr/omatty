// The frame's chrome (#174): a header row, a rule and hairline columns where
// three rounded boxes used to be. The box around the session pane framed a
// program that draws its own rounded prompt box, and its four border columns
// were 5% of an 80-column window. One hairline per seam gives those columns
// to claude; the header row carries what the boxes' top rules carried.
//
// One rule for the whole screen: an accent vertical line stands on the left
// edge of whatever owns the keyboard. Nothing else on the screen is accent.

package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// hairline, ruleDash and ruleJoint are the three glyphs the chrome is drawn
// with. Box Drawing, one cell wide in every width table.
const hairline, ruleDash, ruleJoint = "│", "─", "┼"

// keyboardEdge names the hairline the accent stands on.
type keyboardEdge int

const (
	edgeNone   keyboardEdge = iota // nothing to type into
	edgePane                       // a terminal, or a modal drawn in its pane
	edgeReview                     // the review column
)

// keyboardEdge is the seam whose right-hand column owns the keys now. The
// order is the routing order: a modal takes the keys from everything, the
// review column takes them from the terminal (#95, #21). It is decided here
// rather than per box because a modal takes the keyboard without clearing
// review.Focused, and deciding it twice once drew two focused borders (#95).
func (m *Model) keyboardEdge() keyboardEdge {
	if m.modalOpen() {
		return edgePane
	}
	if m.reviewOwnsKeys() {
		return edgeReview
	}
	if m.focusedTerminal() != nil {
		return edgePane
	}
	return edgeNone
}

// hairlineStyle is the accent on the keyboard owner's edge, the hairline
// grey on every other seam.
func hairlineStyle(accent bool) lipgloss.Style {
	if accent {
		return lipgloss.NewStyle().Foreground(colorAccent)
	}
	return lipgloss.NewStyle().Foreground(colorHairline)
}

// hairlineColumn is one hairline h rows tall, a column for JoinHorizontal.
// Each cell is rendered on its own so a line of the frame carries the cell's
// SGR whole, which is also what lets a test find it.
func hairlineColumn(accent bool, h int) string {
	cell := hairlineStyle(accent).Render(hairline)
	return strings.TrimSuffix(strings.Repeat(cell+"\n", h), "\n")
}

// segment is one column's share of the header row.
type segment struct {
	title string
	width int
	owns  bool // the column owns the keyboard: its title is ink, not muted
}

// headerRow lays the segments across the window with a hairline cell between
// each pair. A title is padded and cut to its column, so the row is exactly
// the frame's width whatever a title says (#35). The hairline cells here are
// never accent: the accent runs from the rule down, not through the chrome.
func headerRow(segs []segment) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, segmentStyle(s.owns).Render(fitLine(" "+s.title, s.width)))
	}
	return strings.Join(parts, hairlineStyle(false).Render(hairline))
}

// segmentStyle is ink and bold for the keyboard owner's title, muted for the
// rest, so the header row says where a keystroke lands as the hairline does.
func segmentStyle(owns bool) lipgloss.Style {
	if owns {
		return headerStyle
	}
	return mutedStyle
}

// ruleRow is the line under the header row: dashes across every column and a
// joint under each hairline.
func ruleRow(segs []segment) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, strings.Repeat(ruleDash, s.width))
	}
	return hairlineStyle(false).Render(strings.Join(parts, ruleJoint))
}
