// t keeps your place (#765). Flipping between the whole session and this turn
// used to put the cursor back at the top, so reading a line and asking "did
// this turn do that?" meant scrolling back to it on every flip.
//
// The anchor is the line's path, new-side number and text: the two scopes are
// diffs against different bases, so hunk headers and old-side numbers differ
// between them while the new side - the file as it is now - is the same file.
// That is review.sameLine's rule, and invariant 7's point: content, not row
// numbers. A removed line has no new side, so it anchors on its file.

package app

import dreview "github.com/WilsonSousajr/omatty/internal/domain/review"

// scopeAnchor is where the cursor was when t was pressed, waiting for the other
// scope's rows. session guards against a load that lands for another session.
type scopeAnchor struct {
	session, path, text string
	newNo               int
}

// takeScopeAnchor remembers the row under the cursor before t flips the scope.
func (m *Model) takeScopeAnchor() {
	m.scopeAnchor = nil
	if m.review.DiffList.Cursor >= len(m.review.Entries) {
		return
	}
	e, d := m.review.Entries[m.review.DiffList.Cursor], m.shownDiff()
	a := &scopeAnchor{session: m.review.SessionID, path: d.Files[e.Pos.File].Path}
	if e.Kind == dreview.EntryLine || e.Kind == dreview.EntryComment {
		l := d.LineAt(e.Pos)
		a.text, a.newNo = l.Text, l.NewNo
	}
	m.scopeAnchor = a
}

// restoreScopeAnchor puts the cursor back on the anchor once the scope it
// flipped to has rows, and forgets it. Before then - the turn still loading -
// it waits, because there is nothing to land on.
func (m *Model) restoreScopeAnchor() {
	a := m.scopeAnchor
	if a == nil || a.session != m.review.SessionID || !m.scopeReady() {
		return
	}
	m.scopeAnchor = nil
	m.review.DiffList.Cursor = a.rowIn(m.review.Entries, m.shownDiff())
}

// scopeReady reports whether the shown scope's diff has answered.
func (m *Model) scopeReady() bool {
	if m.review.Scope == scopeTurn {
		return m.review.TurnReady
	}
	return m.review.DiffReady
}

// rebuildTurnEntries is rebuildEntries for a turn load that just landed, the
// moment a flip to the turn has rows to put the cursor back on.
func (m *Model) rebuildTurnEntries() {
	m.rebuildEntries()
	m.restoreScopeAnchor()
}

// rowIn is the anchor's row in entries: the same line, else its file's header,
// else the nearest file after it, else the last file, else the top.
func (a *scopeAnchor) rowIn(entries []dreview.Entry, d dreview.Diff) int {
	if i, ok := a.lineRow(entries, d); ok {
		return i
	}
	return a.fileRow(entries, d)
}

// lineRow finds the same line: path, new-side number and text.
func (a *scopeAnchor) lineRow(entries []dreview.Entry, d dreview.Diff) (int, bool) {
	if a.newNo == 0 {
		return 0, false
	}
	for i, e := range entries {
		if e.Kind != dreview.EntryLine || d.Files[e.Pos.File].Path != a.path {
			continue
		}
		if l := d.LineAt(e.Pos); l.NewNo == a.newNo && l.Text == a.text && l.Kind != dreview.LineRemoved {
			return i, true
		}
	}
	return 0, false
}

// fileRow is the anchor's file header, or the first file sorting after it -
// git lists files by path - or the last file when none does.
func (a *scopeAnchor) fileRow(entries []dreview.Entry, d dreview.Diff) int {
	last := 0
	for i, e := range entries {
		if e.Kind != dreview.EntryFile {
			continue
		}
		if d.Files[e.Pos.File].Path >= a.path {
			return i
		}
		last = i
	}
	return last
}
