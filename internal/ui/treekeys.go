// The tree and preview keymaps, kept apart from the loading and reading
// half of the column so each file stays a single responsibility (#195).

package ui

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

// treeActionKey runs enter, r, / and a, reporting whether key was one.
func (m *Model) treeActionKey(key string) (tea.Cmd, bool) {
	switch key {
	case "enter":
		return m.openTreeNode(), true
	case "r":
		return m.loadFiles(m.review.SessionID), true
	case "/":
		m.review.Filter.Active = true
		return nil, true
	case "a":
		return m.attachSelected(), true
	}
	return nil, false
}

// treeCursorKey moves the cursor for j/k, reporting whether key was one.
func (m *Model) treeCursorKey(key string) bool {
	switch key {
	case "j", "down":
		m.moveTreeCursor(1)
	case "k", "up":
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
	last := previewLast(m.review.Preview, m.reviewRows())
	switch key {
	case "j", "down":
		m.review.PreviewOffset = min(m.review.PreviewOffset+1, last)
	case "k", "up":
		m.review.PreviewOffset = max(m.review.PreviewOffset-1, 0)
	case "a":
		return m.attachPath(m.review.Preview.Path, false)
	case "esc", "ctrl+c":
		m.review.View, m.review.ColOffset = ViewTree, 0
	default:
		m.panKey(key)
	}
	return nil
}
