package ui

import (
	"fmt"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/coverage"
	"github.com/WilsonSousajr/omatty/internal/review"
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
	default:
		lines = m.reviewBody(w, h)
	}
	// Which column owns the keys - and so wears the accent hairline - is
	// decided once, in keyboardEdge (#174); the title is the header row's.
	return fitBlock(lines, w, h)
}

// reviewTitle names what the column is showing, so a glance at the top row
// says which of the three views has the keys.
func (m *Model) reviewTitle() string {
	return m.viewTitle() + m.panMarker()
}

func (m *Model) viewTitle() string {
	switch m.review.View {
	case ViewTree:
		return "files · " + m.sessionTitle(m.review.SessionID) + m.filterMarker()
	case ViewPreview:
		return m.review.Preview.Path
	case ViewGate:
		return "gate · " + m.sessionTitle(m.review.SessionID)
	}
	return fmt.Sprintf("diff · %d files · %d comments%s",
		len(m.review.Diff.Files), m.commentsFor(m.review.SessionID).Len(), pairingNote(m.review.Diff))
}

// pairingNote is the word a diff that changed source and no tests is worth
// (#257), and nothing at all for the other three outcomes: a remark that
// appeared on most diffs would be decoration, and the eye would learn to skip
// it.
//
// It is a remark, not a gate. Nothing is blocked, nothing turns red, and S
// sends no more than it did. M9's line holds - omatty reports, the operator
// decides - and this is the cheapest possible place to test that line, because
// a flag is exactly the kind of thing that grows teeth later.
//
// Computed per frame rather than cached: Pair reads paths, and only a Rust
// file's hunks, so it costs less than laying out the rows underneath it.
func pairingNote(d review.Diff) string {
	if review.Pair(d) != review.PairingUnpaired {
		return ""
	}
	return " · ⚠ no tests"
}

// filterMarker names the filter in force, so a short listing says why.
func (m *Model) filterMarker() string {
	if m.review.Filter.Query == "" {
		return ""
	}
	return " /" + m.review.Filter.Query
}

// reviewBody is the error, the empty-state line, or the scrolled rows with the
// editor beneath them.
func (m *Model) reviewBody(w, rows int) []string {
	if m.review.Err != "" {
		return []string{errorStyle.Render(fitLine(m.review.Err, w))}
	}
	if len(m.review.Entries) == 0 {
		return []string{mutedStyle.Render("no changes")}
	}
	if !m.review.Note.Active {
		return m.renderEntries(w, rows)
	}
	out := m.renderEntries(w, rows-1)
	return append(out, editLine("note", m.review.Note.Buffer, w))
}

// renderEntries draws the rows-high window around the cursor. The offset is
// recomputed here rather than trusted, because a resize can shrink rows after
// the cursor last moved.
func (m *Model) renderEntries(w, rows int) []string {
	off := ScrollOffset(m.review.Cursor, m.review.Offset, rows)
	end := min(off+rows, len(m.review.Entries))
	comments := m.commentsFor(m.review.SessionID).All()
	out := make([]string, 0, rows)
	for i := off; i < end; i++ {
		e := m.review.Entries[i]
		out = append(out, m.renderEntry(e, i == m.review.Cursor, w, comments))
	}
	return out
}

// renderEntry draws one row; the cursor row is reversed.
func (m *Model) renderEntry(e review.Entry, cursor bool, w int, comments []review.Comment) string {
	text := m.fitContent(m.entryText(e, comments), w)
	if cursor {
		return cursorStyle.Render(text)
	}
	return entryStyle(e, m.review.Diff).Render(text)
}

// entryText is the plain text of a row before styling.
//
// A method since #255: what a row says now depends on the coverage overlay the
// session's last gate loaded, and the overlay is the model's.
func (m *Model) entryText(e review.Entry, comments []review.Comment) string {
	switch e.Kind {
	case review.EntryFile:
		return m.fileHeading(e.Pos.File)
	case review.EntryHunk:
		return expandTabs(e.Text)
	case review.EntryComment:
		return "  >> " + comments[e.Comment].Note
	case review.EntryOrphan:
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
func (m *Model) linePrefix(e review.Entry) string {
	if m.uncovered(e) {
		return uncoveredMark
	}
	return signPrefix(m.review.Diff.LineAt(e.Pos).Kind)
}

// uncoveredMark is what an added line no test covers draws instead of its +.
const uncoveredMark = "!"

// uncovered reports whether e is an added line the overlay has a verdict for
// and that verdict is "never ran".
//
// Three states, and the third is the one that earns the check its shape: a
// line the profile does not mention has no verdict at all - it is a
// declaration, a brace, a comment - and silence is never a claim. Only added
// lines are asked about: a context line's coverage is not this change's
// business, and a removed line is not in the tree the profile describes.
func (m *Model) uncovered(e review.Entry) bool {
	if e.Kind != review.EntryLine {
		return false
	}
	line := m.review.Diff.LineAt(e.Pos)
	if line.Kind != review.LineAdded {
		return false
	}
	covered, known := m.overlay().Files[m.review.Diff.Files[e.Pos.File].Path].Lines[line.NewNo]
	return known && !covered
}

// overlay is the coverage the session under review last loaded, or the zero
// profile - which answers "no verdict" for every line, so no caller needs a
// nil check.
func (m *Model) overlay() coverage.Profile {
	return m.covers[m.review.SessionID]
}

func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", strings.Repeat(" ", tabWidth))
}

func signPrefix(k review.LineKind) string {
	switch k {
	case review.LineAdded:
		return "+"
	case review.LineRemoved:
		return "-"
	}
	return " "
}

// fileHeading is "path +a -b", with both names for a rename, a note for a
// binary whose lines nobody can read, and the count of added lines no test
// covers - so a long diff says where to look without being scrolled (#255).
func (m *Model) fileHeading(fi int) string {
	f := m.review.Diff.Files[fi]
	name := f.Path
	if f.Status == review.FileRenamed {
		name = f.OldPath + " → " + f.Path
	}
	if f.Binary {
		return name + " (binary)"
	}
	a, r := f.Counts()
	return fmt.Sprintf("%s +%d -%d%s", name, a, r, m.uncoveredNote(fi))
}

// uncoveredNote counts what the markers under this header will say, and says
// nothing at all when there is nothing to report - a note on every file would
// be decoration rather than a finding.
func (m *Model) uncoveredNote(fi int) string {
	lines := m.overlay().Files[m.review.Diff.Files[fi].Path].Lines
	if len(lines) == 0 {
		return ""
	}
	n := 0
	for _, h := range m.review.Diff.Files[fi].Hunks {
		n += uncoveredInHunk(h, lines)
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("  %d uncovered", n)
}

// uncoveredInHunk is how many of a hunk's added lines the overlay says never
// ran, counted the same way linePrefix marks them.
func uncoveredInHunk(h review.Hunk, lines map[int]bool) int {
	n := 0
	for _, l := range h.Lines {
		if covered, known := lines[l.NewNo]; l.Kind == review.LineAdded && known && !covered {
			n++
		}
	}
	return n
}
