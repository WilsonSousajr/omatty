// The review column's tree and preview views: listing a session's worktree,
// walking it, and reading one file without leaving omatty (#24).

package ui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// loadFiles lists the session's worktree off the Update goroutine: git on a
// large tree takes long enough to stall the frame, exactly as for the diff.
// The diff view needs no listing, so opening it costs no git call.
func (m *Model) loadFiles(id string) tea.Cmd {
	sess, ok := m.session(id)
	if !ok || m.review.View == ViewDiff {
		return nil
	}
	list := m.files
	return func() tea.Msg {
		paths, err := list(sess.Dir)
		return FilesLoadedMsg{SessionID: id, Paths: paths, Err: err}
	}
}

// onFilesLoaded builds the tree, unless the column closed or moved to another
// session while git was running.
func (m *Model) onFilesLoaded(msg FilesLoadedMsg) tea.Cmd {
	if !m.review.Open || msg.SessionID != m.review.SessionID {
		return nil
	}
	if msg.Err != nil {
		slog.Warn("listing files", "session", msg.SessionID, "err", msg.Err)
		m.review.TreeErr = msg.Err.Error()
		return nil
	}
	m.review.TreeErr = ""
	m.review.Tree = review.NewTree(msg.Paths, m.touched())
	m.moveTreeCursor(0)
	return nil
}

// touched is the set of paths the loaded diff changes, which is what puts a
// mark beside a file in the tree. The diff and the listing arrive
// independently, so the tree is rebuilt whenever either lands.
func (m *Model) touched() map[string]bool {
	out := map[string]bool{}
	for _, f := range m.review.Diff.Files {
		out[f.Path] = true
	}
	return out
}

// retouchTree re-marks an already-listed tree when a diff lands after it,
// which is the usual order: `git ls-files` returns before `git diff` (#24).
func (m *Model) retouchTree() {
	if m.review.Tree != nil {
		m.review.Tree.Retouch(m.touched())
	}
}

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

// treeRows is the visible listing, empty until it has been loaded.
func (m *Model) treeRows() []review.TreeNode {
	if m.review.Tree == nil {
		return nil
	}
	return m.review.Tree.Visible()
}

// moveTreeCursor moves the cursor by delta and scrolls to keep it on screen.
// A delta of 0 re-clamps it after the listing under it changed.
func (m *Model) moveTreeCursor(delta int) {
	n := len(m.treeRows())
	if n == 0 {
		m.review.TreeCursor, m.review.TreeOffset = 0, 0
		return
	}
	m.review.TreeCursor = min(max(m.review.TreeCursor+delta, 0), n-1)
	m.review.TreeOffset = ScrollOffset(m.review.TreeCursor, m.review.TreeOffset, m.reviewRows())
}

// openTreeNode collapses or expands a directory, or previews a file.
func (m *Model) openTreeNode() tea.Cmd {
	rows := m.treeRows()
	if m.review.TreeCursor >= len(rows) {
		return nil
	}
	n := rows[m.review.TreeCursor]
	if n.IsDir {
		m.review.Tree.Toggle(n.Path)
		m.moveTreeCursor(0)
		return nil
	}
	m.previewFile(n.Path)
	return nil
}

// previewFile reads synchronously rather than as a command: the read is
// bounded to 256 KiB, which is faster than a frame.
func (m *Model) previewFile(rel string) {
	sess, ok := m.session(m.review.SessionID)
	if !ok {
		return
	}
	p, err := m.preview(sess.Dir, rel)
	if err != nil {
		slog.Warn("previewing a file", "session", m.review.SessionID, "path", rel, "err", err)
		m.lastErr = err.Error()
		return
	}
	m.review.Preview, m.review.PreviewOffset, m.review.View = p, 0, ViewPreview
	m.review.ColOffset = 0 // a new file opens at its left edge, not mid-line (#94)
}

// onPreviewKey scrolls the preview; esc returns to the tree, which is where
// the operator came from, rather than all the way to the terminal.
func (m *Model) onPreviewKey(key string) tea.Cmd {
	last := max(len(m.review.Preview.Lines)-m.reviewRows(), 0)
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
