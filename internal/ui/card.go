// The session card (#176): two lines per session where one row was, in the
// anatomy ade's TUI fixes for every card so the renderer and the mouse can
// share one height. Line one names it, line two says where it is and what
// it did.

package ui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// The card's columns at SidebarWidth 29: 28 of content, the last one blank
// so nothing touches the hairline, and gutterCols after the rail so nothing
// touches the cursor bar either (#498). Line one spends the rail, the gutter,
// the glyph, two spaces, the age and the blank; the title gets 18, three more
// than #155's fifteen. Line two spends the rail, the gutter, two spaces and
// the blank; the branch and the diffstat share the 23 between (#180). It was
// 16 until #410 gave the activity lane's seven columns back to the branch.
//
// #498 widened the sidebar by exactly gutterCols, so both budgets are what
// they were: the gutter is paid for out of the pane's column, never out of a
// title or a branch.
const (
	cardCols  = sidebarContentCols
	ageCols   = 4
	titleCols = cardCols - 1 - gutterCols - 1 - 1 - 1 - ageCols - 1
	metaCols  = cardCols - 1 - gutterCols - 2 - 1
)

// cardGutter is the blank column between the rail and a card's content. The
// rail itself stays flush left: it is the cursor, and a cursor indented off
// the edge it marks is not marking it (#498).
var cardGutter = strings.Repeat(" ", gutterCols)

// rail is the cursor: an accent bar down the left of both lines of the
// selected card, the same idiom as the accent hairline on the keyboard owner
// (#174). A block element, East Asian Ambiguous like the status glyphs, so
// the width rule statusGlyphs describes in style.go covers it too.
const rail = "▎"

// renderRow draws a row's lines: one for a project header, three for a card.
//
//	▎⠹ parser-fix           4m
//	▎  feat/parser-rew… +12 −3
//	▎  ✓✓✓✗ test       88.4%
func (m *Model) renderRow(row Row, now time.Time) []string {
	if row.Session == nil {
		return []string{m.renderHeaderRow(row)}
	}
	r := m.rail(m.isSelected(row.Session.ID)) + cardGutter
	id := row.Session.ID
	return []string{
		r + m.cardTop(row, now),
		r + "  " + m.cardMeta(id) + " ",
		r + "  " + m.cardGate(id) + " ",
	}
}

// renderHeaderRow is a project's one line: the rail column, the name in muted,
// then its open counts right-aligned (#395). The rail is drawn on a header the
// cursor rests on: an empty project's (#158) or a folded one's (#505).
//
// A project with sessions carries the tree's fold arrow before its name, ▾
// open and ▸ folded (#505). A folded one adds, after the counts, how many
// sessions it hides and the loudest of their glyphs - last on the line, so
// the glyph's own colour ends the line instead of cutting the muted run.
//
// A project with nothing known keeps the old single-budget line rather than a
// name one cell shorter beside an empty count: "exactly as it was" is what
// unknown counts promise.
func (m *Model) renderHeaderRow(row Row) string {
	rail := m.rail(m.headerSelected(row.Project)) + cardGutter
	name := m.foldArrow(row) + row.Project
	right := joinSpaced(m.forgeCounts(row.Project), foldSummary(row))
	if right == "" {
		return rail + mutedStyle.Render(fitLine(name, cardCols-1-gutterCols))
	}
	name = fitLine(name, cardCols-1-gutterCols-1-lipgloss.Width(right))
	return rail + mutedStyle.Render(name+" "+right)
}

// headerSelected is whether the cursor rests on project's header.
func (m *Model) headerSelected(project string) bool {
	if p, ok := m.sidebar.SelectedHeader(); ok {
		return p == project
	}
	p, ok := m.sidebar.SelectedFold()
	return ok && p == project
}

// foldArrow is the tree's directory arrow for a project header, or "" for a
// project with nothing to fold.
func (m *Model) foldArrow(row Row) string {
	switch {
	case len(row.Folded) > 0:
		return "▸ "
	case m.hasSessions(row.Project):
		return "▾ "
	}
	return ""
}

// foldSummary is what a folded header says about what it hides: the count,
// then the loudest status glyph. "" on an open header.
func foldSummary(row Row) string {
	if len(row.Folded) == 0 {
		return ""
	}
	return strconv.Itoa(len(row.Folded)) + " " + statusCell(row.Status)
}

// joinSpaced joins the non-empty parts with one space.
func joinSpaced(a, b string) string {
	if a == "" || b == "" {
		return a + b
	}
	return a + " " + b
}

// cardTop is line one past the rail: the glyph, the title, the age.
func (m *Model) cardTop(row Row, now time.Time) string {
	glyph := m.glyphCell(row.Session.ID, row.Status, now)
	title := m.titleStyle(row.Session.ID).Render(fitLine(row.Session.Title, titleCols))
	age := mutedStyle.Render(padLeft(clip(AgeString(now, m.status[row.Session.ID].At), ageCols), ageCols))
	return glyph + " " + title + " " + age + " "
}

// cardMeta is line two's middle: the branch, then the diffstat right-aligned
// (#180). The diffstat is drawn whole and the branch clipped to what remains
// less one separating space; a clean tree gives the branch all 23, and an
// unpolled session gives blanks so the line keeps its width.
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
