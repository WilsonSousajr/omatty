package app

import (
	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"github.com/WilsonSousajr/omatty/internal/tui/theme"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// tabWidth is what a tab becomes before a row is measured. Tabs are expanded
// here rather than left to the renderer: lipgloss.Width counts a tab as one
// cell but draws it as several, so an unexpanded tab made the column wider
// than the width the layout had reserved for it, and the frame tore (#21).
const tabWidth = 4

// renderReview boxes the review column: a title, then whichever view is
// showing - diff rows with their comments and the note editor, the worktree
// tree, or one file's preview (#21, #24).
func (m *Model) renderReview(w, h int) string {
	// The title is in the rule, so every view draws h rows (#128).
	var lines []string
	switch m.review.View {
	case ViewTree:
		lines = m.renderTree(w, h)
	case ViewPreview:
		lines = m.renderPreview(w, h)
	case ViewGate:
		lines = m.renderGate(w, h)
	case ViewTracker:
		lines = m.renderTracker(w, h)
	case ViewTrackerItem:
		lines = m.renderTrackerItem(w, h)
	default:
		lines = m.withFileList(m.reviewBody(m.diffBodyWidth(w), h), h) // the list beside, zoomed (#437)
	}
	// Which column owns the keys - and so wears the accent hairline - is
	// decided once, in keyboardEdge (#174); the title is the header row's.
	return indentBlock(fitBlock(lines, w, h), gutterCols)
}

// indentBlock pushes a rendered block right by n blank columns, so the block
// is n cells wider than it was.
//
// The review column's gutter is applied here, once, rather than in each of
// the six faces (#498): columnWidth already hands every face the narrowed
// budget, so a face added later gets its breathing room for nothing - and
// cannot forget it. The block is sized exactly by fitBlock before the indent,
// which is the bargain joinColumns and joinRows trade on (#174).
func indentBlock(block string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(block, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}

// reviewBody is the error, the empty-state line, or the scrolled rows with the
// editor beneath them.
func (m *Model) reviewBody(w, rows int) []string {
	if m.review.Err != "" {
		return noticeLines([]string{"loading the diff failed:", m.review.Err, logHint}, true, w)
	}
	if m.review.Scope == scopeTurn {
		if lines, isErr := m.turnNotice(); lines != nil {
			return noticeLines(lines, isErr, w)
		}
	}
	if len(m.review.Entries) == 0 {
		return []string{theme.Muted.Render(m.noChanges())}
	}
	if !m.review.Note.Active {
		return m.renderEntries(w, rows)
	}
	out := m.renderEntries(w, rows-1)
	return append(out, editLine(noteLabel(m.review.Note), m.review.Note.Buffer, w))
}

// renderEntries draws the rows-high window around the cursor. The offset is
// recomputed here rather than trusted, because a resize can shrink rows after
// the cursor last moved.
func (m *Model) renderEntries(w, rows int) []string {
	off := ScrollOffset(m.review.DiffList.Cursor, m.review.DiffList.Offset, rows)
	end := min(off+rows, len(m.review.Entries))
	comments := m.commentsFor(m.review.SessionID).All()
	out := make([]string, 0, rows)
	for i := off; i < end; i++ {
		e := m.review.Entries[i]
		out = append(out, m.renderEntry(e, i == m.review.DiffList.Cursor, w, comments))
	}
	return out
}

// renderEntry draws one row; the cursor row is reversed.
func (m *Model) renderEntry(e dreview.Entry, cursor bool, w int, comments []dreview.Comment) string {
	text := m.fitRow(e, comments, w)
	if cursor {
		return theme.Cursor.Render(text)
	}
	if e.Kind == dreview.EntryComment && !comments[e.Comment].Sent.IsZero() {
		return theme.Muted.Render(text) // sent: context for this turn, not a to-do (#335)
	}
	if e.Kind == dreview.EntryLine {
		return m.styledLine(e, w) // syntax and changed words (#435)
	}
	return entryStyle(e, m.shownDiff()).Render(text)
}

// fitRow draws one row into the column. A file header fits itself and stays
// put; every other row is panned and then cut (#291).
//
// The header does not pan because it is a header: pan.go says why the titles
// keep a plain fitLine, and a header that slides sideways with the body reads
// as a broken frame. Once it fits itself there is nothing behind it to reveal,
// and panning would slide the count off the left instead of the right.
func (m *Model) fitRow(e dreview.Entry, comments []dreview.Comment, w int) string {
	if e.Kind == dreview.EntryFile {
		return fitLine(m.headerWithPlace(e.Pos.File, w), w)
	}
	return m.fitContent(m.entryText(e, comments), w)
}

// entryText is the plain text of a panned row before styling. A file header is
// not one of them: it fits itself, in fileheader.go.
//
// A method since #255: what a row says now depends on the coverage overlay the
// session's last gate loaded, and the overlay is the model's.
func (m *Model) entryText(e dreview.Entry, comments []dreview.Comment) string {
	switch e.Kind {
	case dreview.EntryHunk:
		return expandTabs(e.Text)
	case dreview.EntryComment:
		if c := comments[e.Comment]; !c.Sent.IsZero() {
			return "  >> (sent " + c.Sent.Format("15:04") + ") " + c.Note
		}
		return "  >> " + comments[e.Comment].Note
	case dreview.EntryOrphan:
		return "  >> (moved) " + comments[e.Comment].Note
	}
	return m.linePrefix(e) + expandTabs(e.Text)
}

// linePrefix is the row's first cell: its sign, or the uncovered marker where
// the overlay says an added line never ran (#255).
//
// The marker takes the sign's cell rather than adding a gutter column of its
// own, deliberately. A new column would shift every row of every diff,
// including the files a profile says nothing about, and an overlay is a remark
// on the diff rather than a redesign of it. The line keeps its added colour, so
// the row still reads as an addition - one nothing exercised.
func (m *Model) linePrefix(e dreview.Entry) string {
	if m.uncovered(e) {
		return uncoveredMark
	}
	return signPrefix(m.shownDiff().LineAt(e.Pos).Kind)
}

func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", strings.Repeat(" ", tabWidth))
}

func signPrefix(k dreview.LineKind) string {
	switch k {
	case dreview.LineAdded:
		return "+"
	case dreview.LineRemoved:
		return "-"
	}
	return " "
}

// noChanges is the empty diff's line, which says which diff was empty.
func (m *Model) noChanges() string {
	if m.review.Scope == scopeTurn {
		return "no changes this turn"
	}
	return "no changes"
}

// noticeLines draws a notice wrapped to the column rather than cut at its
// edge: a cut line lost the part of an error that says what went wrong
// (#351), and git's stderr can be several lines of its own.
func noticeLines(lines []string, isErr bool, w int) []string {
	style := theme.Muted
	if isErr {
		style = theme.Error
	}
	var out []string
	for _, l := range lines {
		for _, part := range strings.Split(ansi.Wrap(l, w, " "), "\n") {
			out = append(out, style.Render(fitLine(part, w)))
		}
	}
	return out
}

// noteLabel says which of the note editor's two prompts is showing (#339).
func noteLabel(n noteEditor) string {
	if n.Stage == stageFragment {
		return "fragment"
	}
	return "note"
}
