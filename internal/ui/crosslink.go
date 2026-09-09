// o links the diff and the preview both ways (#200): from a hunk line to the
// whole file around it, and from a file back to what changed in it. The data
// already lines up: an entry carries its Position, a Line its numbers.

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// openPreviewAtCursor is o in the diff: the preview of the entry's file with
// the entry's line on the top row. The listing is fetched if the column
// never had one, because esc from the preview lands on the tree and a column
// opened with ctrl+o d would otherwise show #131's spinner for good.
func (m *Model) openPreviewAtCursor() tea.Cmd {
	e, ok := m.cursorEntry()
	if !ok {
		return nil
	}
	f := m.review.Diff.Files[e.Pos.File]
	if f.Status == review.FileDeleted {
		m.previewDeleted(f.Path)
		return nil
	}
	m.previewFile(f.Path)
	if m.review.View == ViewPreview {
		m.review.PreviewOffset = m.previewOffsetFor(e)
	}
	return m.loadFilesIfMissing(m.review.SessionID)
}

// previewOffsetFor is the top row for an entry: its line's new number, or
// the old one for a removed line, clamped the way j is; a header opens at
// the top. renderPreview clamps again so a line near the end is not the top
// row - the file simply ends.
func (m *Model) previewOffsetFor(e review.Entry) int {
	if e.Kind == review.EntryFile || e.Kind == review.EntryOrphan {
		return 0
	}
	l := m.review.Diff.LineAt(e.Pos)
	no := l.NewNo
	if no == 0 {
		no = l.OldNo
	}
	return min(max(no-1, 0), previewLast(m.review.Preview, m.reviewRows()))
}

// openDiffAtPreview is o in the preview: the diff with its cursor on the
// line at the preview's top row, or on the file's header when no diff line
// is there, so o, o from a hunk line is a round trip. A file the diff does
// not touch has nowhere to go, and the footer says so.
func (m *Model) openDiffAtPreview() tea.Cmd {
	target, ok := m.entryForPreview()
	if !ok {
		m.lastErr = m.review.Preview.Path + " is not in this diff"
		return nil
	}
	m.review.View, m.review.ColOffset, m.review.Cursor = ViewDiff, 0, target
	m.review.Offset = ScrollOffset(target, m.review.Offset, m.reviewRows())
	m.contentChanged()
	return nil
}

// entryForPreview finds the diff line whose new number is the preview's top
// row, falling back to the file's header entry.
func (m *Model) entryForPreview() (int, bool) {
	header, want := -1, m.review.PreviewOffset+1
	for i, e := range m.review.Entries {
		if m.review.Diff.Files[e.Pos.File].Path != m.review.Preview.Path {
			continue
		}
		if e.Kind == review.EntryFile {
			header = i
		}
		if e.Kind == review.EntryLine && m.review.Diff.LineAt(e.Pos).NewNo == want {
			return i, true
		}
	}
	return header, header >= 0
}
