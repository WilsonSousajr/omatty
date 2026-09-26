// The tree's type-to-filter line (#198): / opens it inside the focused column,
// so the leader router is not involved (invariant 1); the session switcher's
// list does the same with internal/fuzzy.

package ui

import tea "charm.land/bubbletea/v2"

// onFilterKey edits the query with the shared editline and refilters on every
// change; enter keeps the query and hands the keys back to the list, esc
// clears it and gives the full listing back.
func (m *Model) onFilterKey(msg tea.KeyPressMsg) tea.Cmd {
	buffer, action := editKey(m.review.Filter.Query, msg)
	switch action {
	case editCommit:
		m.review.Filter.Active = false
	case editCancel:
		m.review.Filter.Active = false
		m.setViewFilter("")
	default:
		m.setViewFilter(buffer)
	}
	return nil
}

// setViewFilter applies the query to whichever list has the line: the tree's
// listing, or the tracker's rows (#399). One filter line, two lists - which is
// why the query lives on ReviewPane rather than in either of them.
func (m *Model) setViewFilter(query string) {
	if m.review.View == ViewTracker {
		m.setTrackerFilter(query)
		return
	}
	m.setTreeFilter(query)
}

// setTreeFilter applies query to the tree and re-clamps the cursor and the
// pan, since the visible set changed under both (#133).
func (m *Model) setTreeFilter(query string) {
	m.review.Filter.Query = query
	if m.review.Tree != nil {
		m.review.Tree.SetFilter(query)
	}
	m.contentChanged()
	m.moveTreeCursor(0)
}
