// The session card (#176): two lines per session where one row was, in the
// anatomy ade's TUI fixes for every card so the renderer and the mouse can
// share one height. Line one names it, line two says where it is and what
// it did.

package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/review"
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

// renderRow draws a row's lines: one for a project header, three for a card.
//
//	▎● parser-fix           4m
//	▎  main      +12 −3 ▁▃▇█▅▂
//	▎  ✓✓✓✗ test       88.4%
func (m *Model) renderRow(row Row, now time.Time) []string {
	if row.Session == nil {
		return []string{m.renderHeaderRow(row.Project)}
	}
	r := m.rail(m.isSelected(row.Session.ID))
	id := row.Session.ID
	return []string{
		r + m.cardTop(row, now),
		r + "  " + m.cardMeta(id) + " " + m.renderLane(id) + " ",
		r + "  " + m.cardGate(id) + " ",
	}
}

// renderHeaderRow is a project's one line: the rail column, the name in muted,
// then its open counts right-aligned (#395). The rail is drawn only on an empty
// project the cursor rests on, the one header that can be selected (#158).
//
// A project with nothing known keeps the old single-budget line rather than a
// name one cell shorter beside an empty count: "exactly as it was" is what
// unknown counts promise.
func (m *Model) renderHeaderRow(project string) string {
	p, ok := m.sidebar.SelectedHeader()
	rail := m.rail(ok && p == project)
	counts := m.forgeCounts(project)
	if counts == "" {
		return rail + mutedStyle.Render(fitLine(project, cardCols-1))
	}
	name := fitLine(project, cardCols-1-1-lipgloss.Width(counts))
	return rail + mutedStyle.Render(name+" "+counts)
}

// cardTop is line one past the rail: the glyph, the title, the age.
func (m *Model) cardTop(row Row, now time.Time) string {
	glyph := statusCell(row.Status)
	title := m.titleStyle(row.Session.ID).Render(fitLine(row.Session.Title, titleCols))
	age := mutedStyle.Render(padLeft(clip(AgeString(now, m.status[row.Session.ID].At), ageCols), ageCols))
	return glyph + " " + title + " " + age + " "
}

// cardMeta is line two's middle: the branch, then the diffstat right-aligned
// (#180). The diffstat is drawn whole and the branch clipped to what remains
// less one separating space; a clean tree gives the branch all sixteen, and
// an unpolled session gives blanks so the lane keeps its place.
func (m *Model) cardMeta(id string) string {
	st, ok := m.repoStat[id]
	if !ok {
		return strings.Repeat(" ", metaCols)
	}
	left, kind := m.metaLeft(id, st.Branch)
	stat := diffstat(st)
	if kind != metaBranch && lipgloss.Width(left)+1+lipgloss.Width(stat) > metaCols {
		left, stat = squeezePR(left, kind, st)
	}
	if stat == "" {
		return fitLine(left, metaCols)
	}
	return fitLine(left, metaCols-lipgloss.Width(stat)-1) + " " + stat
}

// metaKind is what line two's left side names, which decides what gives way
// when it and the diffstat do not both fit.
type metaKind int

const (
	metaBranch     metaKind = iota // cut to keep the diffstat (#180)
	metaOpenPR                     // keeps its CI mark, and the diffstat its place (#357)
	metaFinishedPR                 // "#349 merged": the diffstat gives way (#310)
)

// squeezePR fits a pull request label beside the diffstat. A finished one
// wins outright - "#349 merged" says more about a finished session than its
// line counts (#310). An open one is an active card, where the counts matter:
// the label drops the space before its mark, and the diffstat sheds whole
// parts rather than being cut mid-number, which would read as a different
// count. The label, and so the CI verdict, is never cut (#357).
func squeezePR(label string, kind metaKind, st review.Stat) (string, string) {
	if kind == metaFinishedPR {
		return label, ""
	}
	label = strings.Replace(label, " ", "", 1)
	return label, diffstatWithin(st, metaCols-lipgloss.Width(label)-1)
}

// diffstatWithin is the diffstat in at most w cells: whole, then the added
// count alone, then nothing.
func diffstatWithin(st review.Stat, w int) string {
	if full := diffstat(st); lipgloss.Width(full) <= w {
		return full
	}
	if added := addedStyle.Render("+" + KString(st.Added)); lipgloss.Width(added) <= w {
		return added
	}
	return ""
}

// metaLeft is the pull request the session's branch has, or the branch.
func (m *Model) metaLeft(id, branch string) (string, metaKind) {
	sess, ok := m.session(id)
	if !ok {
		return branch, metaBranch
	}
	pr, found := m.prFor(sess)
	switch {
	case !found:
		return branch, metaBranch
	case pr.State == forge.Open:
		return m.prLabel(pr, sess.Project), metaOpenPR
	}
	return m.prLabel(pr, sess.Project), metaFinishedPR
}

// diffstat is "+12 −3" in the diff colours, "" for a clean tree - never
// "+0 −0" - with KString keeping a large count inside the budget (#180).
func diffstat(st review.Stat) string {
	if st.Added == 0 && st.Removed == 0 {
		return ""
	}
	return addedStyle.Render("+"+KString(st.Added)) + " " + removedStyle.Render("−"+KString(st.Removed))
}

// rail is the accent bar on the selected card, a blank column on the rest.
func (m *Model) rail(selected bool) string {
	if selected {
		return accentStyle.Render(rail)
	}
	return " "
}

// titleStyle is ink and bold on the selected card, muted on a stopped one,
// text on the rest. A stopped card keeps its glyph and age - those come from
// the transcript (invariant 2) - and reads as asleep by its title alone, so
// no legend and no layout change (#318).
func (m *Model) titleStyle(id string) lipgloss.Style {
	if m.isSelected(id) {
		return headerStyle
	}
	if m.terms[id] == nil {
		return mutedStyle
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
