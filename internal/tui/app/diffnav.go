// Moving through a diff by file and by hunk, and folding a file (#436).
//
// Two levels, as diffview, diffnav and lazygit have them: ] and [ walk the
// files, n and N the hunks inside and across them. enter on a file's header
// folds the file to that one row - a viewing state, keyed by path so a reload
// keeps it, and never persisted. #337's reviewed mark is the durable version.

package app

import (
	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"strconv"

	"charm.land/lipgloss/v2"
)

// diffNavKey runs ], [, n, N and enter, reporting whether key was one.
func (m *Model) diffNavKey(key string) bool {
	switch {
	case is(key, diffBind.NextFile):
		m.jumpEntry(1, dreview.EntryFile)
	case is(key, diffBind.PrevFile):
		m.jumpEntry(-1, dreview.EntryFile)
	case is(key, diffBind.NextHunk):
		m.jumpEntry(1, dreview.EntryHunk)
	case is(key, diffBind.PrevHunk):
		m.jumpEntry(-1, dreview.EntryHunk)
	case is(key, diffBind.Fold):
		m.toggleFileFold()
	default:
		return false
	}
	return true
}

// jumpEntry moves the cursor to the next entry of kind in dir, staying put
// when there is none that way.
func (m *Model) jumpEntry(dir int, kind dreview.EntryKind) {
	for i := m.review.DiffList.Cursor + dir; i >= 0 && i < len(m.review.Entries); i += dir {
		if m.review.Entries[i].Kind == kind {
			m.moveReviewCursor(i - m.review.DiffList.Cursor)
			return
		}
	}
}

// toggleFileFold folds the file whose header is under the cursor, or opens it.
// The header's own entry does not move, so the cursor stays on it.
func (m *Model) toggleFileFold() {
	e, ok := m.cursorEntry()
	if !ok || e.Kind != dreview.EntryFile {
		return
	}
	path := m.shownDiff().Files[e.Pos.File].Path
	if m.review.FoldedFiles == nil {
		m.review.FoldedFiles = map[string]bool{}
	}
	m.review.FoldedFiles[path] = !m.review.FoldedFiles[path]
	m.rebuildEntries()
}

// withoutFolded drops every entry under a folded file's header.
func (m *Model) withoutFolded(entries []dreview.Entry, d dreview.Diff) []dreview.Entry {
	if len(m.review.FoldedFiles) == 0 {
		return entries
	}
	out := entries[:0]
	for _, e := range entries {
		if e.Kind == dreview.EntryFile || !m.review.FoldedFiles[d.Files[e.Pos.File].Path] {
			out = append(out, e)
		}
	}
	return out
}

// foldNote is what a folded header adds to its never-given-up note.
func (m *Model) foldNote(fi int) string {
	if m.review.FoldedFiles[m.shownDiff().Files[fi].Path] {
		return " · folded"
	}
	return ""
}

// headerWithPlace is a file's header with its place among the diff's files,
// "2/11", when that costs the header nothing: the place is the first thing a
// header gives up, as the position is in a title (#424).
func (m *Model) headerWithPlace(fi, w int) string {
	f, note := m.shownDiff().Files[fi], m.uncoveredNote(fi)+m.foldNote(fi)
	head := fileHeading(f, note, w)
	place := " " + strconv.Itoa(fi+1) + "/" + strconv.Itoa(len(m.shownDiff().Files))
	if lipgloss.Width(head+place) > w || fileHeading(f, note, w-lipgloss.Width(place)) != head {
		return head
	}
	return head + place
}
