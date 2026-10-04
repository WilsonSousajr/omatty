package app

// toggleTrackerFold is tab in the tracker (#663): fold the section the cursor
// is in down to its heading, or open the folded one it rests on. With many
// open issues the pull requests sat a long scroll below them; the sidebar's
// tab folds a project the same way.
//
// Folding moves the cursor to the next section, or back to the one before
// when there is none below, so it never rests on an open heading (#432).
// Opening puts it on the section's first item. A list with no heading is the
// only list, and has nothing to fold away from.
func (m *Model) toggleTrackerFold() {
	rows := m.trackerRows()
	if m.review.Tracker.Cursor >= len(rows) {
		return
	}
	kind := sectionOf(rows[m.review.Tracker.Cursor])
	if ruleOf(rows, kind) < 0 {
		return
	}
	folded := m.flipTrackerFold(kind)
	rows = m.trackerRows()
	at := ruleOf(rows, kind)
	target := at + 1
	if folded {
		target = nearestStart(sectionStarts(rows), at)
	}
	m.moveTrackerCursor(target - m.review.Tracker.Cursor)
}

// flipTrackerFold folds or opens kind for the tracker's project, and reports
// whether it is folded now. Kept per project and in memory only: moving to
// another project and back keeps it, a restart opens everything, and nothing
// is stored (invariant 9 needs none of it).
func (m *Model) flipTrackerFold(kind trackerKind) bool {
	project := m.review.Tracker.Project
	if m.trackerFolds[project] == nil {
		m.trackerFolds[project] = map[trackerKind]bool{}
	}
	folds := m.trackerFolds[project]
	folds[kind] = !folds[kind]
	return folds[kind]
}

// sectionOf is the list a row belongs to: an item's own, or the one a heading
// heads.
func sectionOf(r trackerRow) trackerKind {
	if r.Kind == rowRule {
		return r.Section
	}
	return r.Kind
}

// ruleOf is the row of kind's heading, or -1 when the list has none.
func ruleOf(rows []trackerRow, kind trackerKind) int {
	for i, r := range rows {
		if r.Kind == rowRule && r.Section == kind {
			return i
		}
	}
	return -1
}

// nearestStart is the first start after at, or the last before it when there
// is none after; at itself when it is the only one.
func nearestStart(starts []int, at int) int {
	best := at
	for _, s := range starts {
		if s > at {
			return s
		}
		if s < at {
			best = s
		}
	}
	return best
}
