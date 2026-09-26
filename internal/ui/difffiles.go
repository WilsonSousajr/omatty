// The diff's file list (#437): zoomed and wide, the diff's files listed beside
// it - diffnav's file tree, diffview's panel, GitHub's "Files changed" - each
// with its status, its path and its counts, and #337's reviewed mark. The two
// move together: the list marks the file the diff's cursor is in, and a click
// on the list moves the diff to that file.

package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// fileListWidth is the list's cells, diffnav's default.
const fileListWidth = 26

// listingFiles reports whether the diff is drawn with its file list.
func (m *Model) listingFiles() bool {
	return m.review.View == ViewDiff && m.zoomed() && m.columnWidth() >= previewMinWidth &&
		len(m.shownDiff().Files) > 0
}

// diffBodyWidth is the diff's share of a column w wide.
func (m *Model) diffBodyWidth(w int) int {
	if m.listingFiles() {
		return w - fileListWidth - len([]rune(previewSeam))
	}
	return w
}

// withFileList lays the file list to the left of the diff's rows.
func (m *Model) withFileList(body []string, h int) []string {
	if !m.listingFiles() {
		return body
	}
	files, bw := m.fileListLines(h), m.diffBodyWidth(m.columnWidth())
	out := make([]string, h)
	for i := range out {
		out[i] = fitLine(lineAt(files, i), fileListWidth) + mutedStyle.Render(previewSeam) + fitLine(lineAt(body, i), bw)
	}
	return out
}

// cursorFile is the file the diff's cursor is in.
func (m *Model) cursorFile() int {
	if e, ok := m.cursorEntry(); ok {
		return e.Pos.File
	}
	return 0
}

// fileListOffset is the first file drawn: the list scrolls to keep the
// cursor's file in it.
func (m *Model) fileListOffset(h int) int {
	return ScrollOffset(m.cursorFile(), 0, h)
}

// fileListLines is the list, h rows from its offset.
func (m *Model) fileListLines(h int) []string {
	files, current := m.shownDiff().Files, m.cursorFile()
	var out []string
	for i := m.fileListOffset(h); i < len(files) && len(out) < h; i++ {
		out = append(out, m.fileListRow(files[i], i == current))
	}
	return out
}

// fileListRow is one file: its reviewed mark and status, its path shortened
// from the front to keep the name (#287), its counts at the right edge. The
// cursor's file is drawn bold.
func (m *Model) fileListRow(f review.File, current bool) string {
	a, r := f.Counts()
	lead := m.reviewMark(f.Path, false) + changeLetter(review.ChangeOf(f.Status)) + " "
	stat := headingStat(a, r)
	room := fileListWidth - lipgloss.Width(lead) - lipgloss.Width(stat)
	path := previewTitle(f.Path, max(room, 1))
	if current {
		path = headerStyle.Render(path)
	}
	pad := strings.Repeat(" ", max(room-lipgloss.Width(path), 0))
	return lead + path + pad + mutedStyle.Render(stat)
}

// clickFileList moves the diff to the file clicked in the list, and reports
// whether the click was on the list at all.
func (m *Model) clickFileList(x, y int) bool {
	if !m.listingFiles() || x < SidebarWidth || x >= SidebarWidth+fileListWidth {
		return false
	}
	fi := m.fileListOffset(m.reviewRows()) + y - reviewTop()
	for i, e := range m.review.Entries {
		if e.Kind == review.EntryFile && e.Pos.File == fi {
			m.moveReviewCursor(i - m.review.DiffList.Cursor)
			m.review.Focused = true
			return true
		}
	}
	return true
}
