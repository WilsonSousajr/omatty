// The tree and preview keymaps, kept apart from the loading and reading
// half of the column so each file stays a single responsibility (#195).

package ui

import tea "charm.land/bubbletea/v2"

// onTreeKey handles a plain keystroke while the tree is shown.
func (m *Model) onTreeKey(key string) tea.Cmd {
	switch key {
	case "j", "down":
		m.moveTreeCursor(1)
	case "k", "up":
		m.moveTreeCursor(-1)
	case "enter":
		return m.openTreeNode()
	case "r":
		return m.loadFiles(m.review.SessionID)
	case "esc", "ctrl+c":
		m.review.Focused = false
	default:
		m.panKey(key)
	}
	return nil
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
