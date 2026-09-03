package ui

import (
	"fmt"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// tabWidth is what a tab becomes before a row is measured. Tabs are expanded
// here rather than left to the renderer: lipgloss.Width counts a tab as one
// cell but draws it as several, so an unexpanded tab made the column wider
// than the width the layout had reserved for it, and the frame tore (#21).
const tabWidth = 4

// renderReview boxes the review column: a title with counts, then the visible
// window of diff rows with their comments, then the note editor.
func (m *Model) renderReview(w, h int) string {
	lines := []string{headerStyle.Render(fitLine(m.reviewTitle(), w))}
	lines = append(lines, m.reviewBody(w, h-1)...)
	return paneBox(m.review.Focused).Render(fitBlock(lines, w, h))
}

func (m *Model) reviewTitle() string {
	return fmt.Sprintf("diff · %d files · %d comments",
		len(m.review.Diff.Files), m.commentsFor(m.review.SessionID).Len())
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
	return append(out, fitLine("note: "+m.review.Note.Buffer+"_", w))
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
	text := fitLine(entryText(e, m.review.Diff, comments), w)
	if cursor {
		return cursorStyle.Render(text)
	}
	return entryStyle(e, m.review.Diff).Render(text)
}

// entryText is the plain text of a row before styling.
func entryText(e review.Entry, d review.Diff, comments []review.Comment) string {
	switch e.Kind {
	case review.EntryFile:
		return fileHeading(d.Files[e.Pos.File])
	case review.EntryHunk:
		return expandTabs(e.Text)
	case review.EntryComment:
		return "  >> " + comments[e.Comment].Note
	case review.EntryOrphan:
		return "  >> (moved) " + comments[e.Comment].Note
	}
	return linePrefix(d.LineAt(e.Pos).Kind) + expandTabs(e.Text)
}

func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", strings.Repeat(" ", tabWidth))
}

func linePrefix(k review.LineKind) string {
	switch k {
	case review.LineAdded:
		return "+"
	case review.LineRemoved:
		return "-"
	}
	return " "
}

// fileHeading is "path +a -b", with both names for a rename and a note for a
// binary, whose lines nobody can read.
func fileHeading(f review.File) string {
	name := f.Path
	if f.Status == review.FileRenamed {
		name = f.OldPath + " → " + f.Path
	}
	if f.Binary {
		return name + " (binary)"
	}
	a, r := f.Counts()
	return fmt.Sprintf("%s +%d -%d", name, a, r)
}
