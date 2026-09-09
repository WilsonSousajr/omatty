// The session card (#176): two lines per session where one row was, in the
// anatomy ade's TUI fixes for every card so the renderer and the mouse can
// share one height. Line one names it, line two says where it is and what
// it did.

package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// The card's columns at SidebarWidth 28: 27 of content, the last one blank
// so nothing touches the hairline. Line one spends the rail, the glyph, two
// spaces, the age and the blank; the title gets 18, three more than #155's
// fifteen. Line two spends the rail, two spaces, a space, the lane and the
// blank; the branch and the diffstat share the 16 between (#180).
const (
	cardCols  = sidebarContentCols
	ageCols   = 4
	titleCols = cardCols - 1 - 1 - 1 - 1 - ageCols - 1
	metaCols  = cardCols - 1 - 2 - 1 - laneCells - 1
)

// rail is the cursor: an accent bar down the left of both lines of the
// selected card, the same idiom as the accent hairline on the keyboard owner
// (#174). A block element like the lane cells, so the East Asian width rule
// in lane.go covers it too.
const rail = "▎"

// renderRow draws a row's lines: one for a project header, two for a card.
//
//	▎● parser-fix           4m
//	▎  main      +12 −3 ▁▃▇█▅▂
func (m *Model) renderRow(row Row, now time.Time) []string {
	if row.Session == nil {
		return []string{m.renderHeaderRow(row.Project)}
	}
	r := m.rail(m.isSelected(row.Session.ID))
	return []string{
		r + m.cardTop(row, now),
		r + "  " + m.cardMeta(row.Session.ID) + " " + m.renderLane(row.Session.ID) + " ",
	}
}

// renderHeaderRow is a project's one line: the rail column, then the name in
// muted. The rail is drawn only on an empty project the cursor rests on, the
// one header that can be selected (#158).
func (m *Model) renderHeaderRow(project string) string {
	p, ok := m.sidebar.SelectedHeader()
	return m.rail(ok && p == project) + mutedStyle.Render(fitLine(project, cardCols-1))
}

// cardTop is line one past the rail: the glyph, the title, the age.
func (m *Model) cardTop(row Row, now time.Time) string {
	glyph := glyphStyle(row.Status).Render(statusGlyph(row.Status))
	title := m.titleStyle(row.Session.ID).Render(fitLine(row.Session.Title, titleCols))
	age := mutedStyle.Render(padLeft(clip(AgeString(now, m.status[row.Session.ID].At), ageCols), ageCols))
	return glyph + " " + title + " " + age + " "
}

// cardMeta is line two's middle. Until #180 wires the branch and the diffstat
// it is blank; the lane keeps its place at the right.
func (m *Model) cardMeta(_ string) string { return strings.Repeat(" ", metaCols) }

// rail is the accent bar on the selected card, a blank column on the rest.
func (m *Model) rail(selected bool) string {
	if selected {
		return accentStyle.Render(rail)
	}
	return " "
}

// titleStyle is ink and bold on the selected card, text on the rest.
func (m *Model) titleStyle(id string) lipgloss.Style {
	if m.isSelected(id) {
		return headerStyle
	}
	return textStyle
}

// isSelected reports whether the cursor rests on session id.
func (m *Model) isSelected(id string) bool {
	sel, ok := m.sidebar.Selected()
	return ok && sel.Session.ID == id
}

// clip cuts s to width without padding it, for a cell that is right-aligned
// afterwards; fitLine would pad first and shift it left.
func clip(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

// padLeft right-aligns s in width, ANSI-aware like padRight.
func padLeft(s string, width int) string {
	if n := width - lipgloss.Width(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}
