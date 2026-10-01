// The tree and preview keymaps, kept apart from the loading and reading
// half of the column so each file stays a single responsibility (#195).

package app

import tea "charm.land/bubbletea/v2"

// onTreeKey handles a plain keystroke while the tree is shown: the cursor
// keys, the action keys, esc, then the shared pan keys.
func (m *Model) onTreeKey(key string) tea.Cmd {
	if m.treeCursorKey(key) {
		return nil
	}
	if cmd, ok := m.treeActionKey(key); ok {
		return cmd
	}
	if key == "esc" || key == "ctrl+c" {
		m.leaveTree()
		return nil
	}
	m.panKey(key)
	return nil
}

// treeActionKey runs enter, r, / and a, then the two marking keys, reporting
// whether key was one.
func (m *Model) treeActionKey(key string) (tea.Cmd, bool) {
	switch {
	case is(key, treeBind.Enter):
		return m.openTreeNode(), true
	case is(key, treeBind.Reload):
		return m.loadFiles(m.review.SessionID), true
	case is(key, treeBind.Filter):
		m.review.Filter.Active = true
		return nil, true
	case is(key, treeBind.Attach):
		return m.attachSelected(), true
	}
	return nil, m.treeMarkKey(key)
}

// treeMarkKey runs the two keys that change what the listing shows about
// itself rather than moving through it: v marks a file read (#337) and g folds
// the generated files in or away (#338). A second table for the reason
// routing.go has several - one switch over every key here is past the
// statement limit, and the split is along a real seam.
func (m *Model) treeMarkKey(key string) bool {
	switch {
	case is(key, treeBind.Read):
		m.toggleReviewed()
	case is(key, treeBind.Generated):
		// Not g, which #424 made "the top" on every face; "." is what lf,
		// ranger, yazi and nnn toggle hidden files with.
		m.toggleGenerated()
	case is(key, treeBind.Changed):
		m.toggleChangedOnly()
	default:
		return false
	}
	return true
}

// treeCursorKey moves the cursor for j/k, reporting whether key was one.
func (m *Model) treeCursorKey(key string) bool {
	switch {
	case is(key, columnBind.Down):
		m.moveTreeCursor(1)
	case is(key, columnBind.Up):
		m.moveTreeCursor(-1)
	default:
		return false
	}
	return true
}

// leaveTree is esc in the list: a kept filter is lifted first, so the
// operator sees the narrowing go before the column does (#198).
func (m *Model) leaveTree() {
	if m.review.Filter.Query != "" {
		m.setTreeFilter("")
		return
	}
	m.review.Focused = false
}

// onPreviewKey scrolls the preview; esc returns to the tree, which is where
// the operator came from, rather than all the way to the terminal.
func (m *Model) onPreviewKey(key string) tea.Cmd {
	switch {
	case is(key, columnBind.Down):
		m.scrollPreview(1)
	case is(key, columnBind.Up):
		m.scrollPreview(-1)
	case is(key, treeBind.Attach):
		return m.attachPath(m.review.Preview.Path, false)
	case is(key, treeBind.Open):
		return m.openDiffAtPreview()
	case is(key, columnBind.Back) || is(key, columnBind.Interrupt):
		m.review.View, m.review.ColOffset = ViewTree, 0
	default:
		m.panKey(key)
	}
	return nil
}
