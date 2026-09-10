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

// segment is one column's share of the header row and the rule.
type segment struct {
	title    string
	width    int
	owns     bool // the column owns the keyboard: its title is ink, not muted
	closable bool // the column can be closed: its rule carries a label and × (#168)
}

// closeGlyph is the one-cell close affordance on a closable column's rule,
// and reviewRuleLabel names the column there. Two diffs can be on screen at
// once - claude's own inside its pane, omatty's review column beside it -
// and nothing said which was which; the pane's rule is plain dashes, the
// column's reads "─ review ─────×", so they are told apart at a glance and
// the one that closes is the one that says so (#168).
const closeGlyph, reviewRuleLabel = "×", "review"

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
// joint under each hairline. Each segment is rendered on its own, so a
// closable one can end in a differently coloured × (#168).
func ruleRow(segs []segment) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, ruleSegment(s))
	}
	return strings.Join(parts, hairlineStyle(false).Render(ruleJoint))
}

// ruleSegment is one column's share of the rule: dashes, or for a closable
// column the label and the × in its last cell. The × is accent when the
// column owns the keys, the rule's grey otherwise: the hue that means focus
// and nothing else (#175).
func ruleSegment(s segment) string {
	if !s.closable || s.width < 1 {
		return hairlineStyle(false).Render(strings.Repeat(ruleDash, s.width))
	}
	return hairlineStyle(false).Render(ruleLabel(s.width-1)) + hairlineStyle(s.owns).Render(closeGlyph)
}

// ruleLabel is width cells of dashes with " review " set into them after one
// dash, or plain dashes when the column is too narrow to carry the name.
func ruleLabel(width int) string {
	label := " " + reviewRuleLabel + " "
	if width < len(label)+2 {
		return strings.Repeat(ruleDash, width)
	}
	return ruleDash + label + strings.Repeat(ruleDash, width-1-len(label))
}
