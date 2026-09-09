// The tree and preview keymaps, kept apart from the loading and reading
// half of the column so each file stays a single responsibility (#195).

package ui

import tea "charm.land/bubbletea/v2"

// onTreeKey handles a plain keystroke while the tree is shown.
func (m *Model) onTreeKey(key string) tea.Cmd {
	if m.treeCursorKey(key) {
		return nil
	}
	switch key {
	case "enter":
		return m.openTreeNode()
	case "r":
		return m.loadFiles(m.review.SessionID)
	case "/":
		m.review.Filter.Active = true
	case "esc", "ctrl+c":
		m.leaveTree()
	default:
		m.panKey(key)
	}
	return nil
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
	case "esc", "ctrl+c":
		m.review.View, m.review.ColOffset = ViewTree, 0
	default:
		m.panKey(key)
	}
	return nil
}
