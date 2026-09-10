// The review column's tree and preview views: listing a session's worktree,
// walking it, and reading one file without leaving omatty (#24).

package ui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/highlight"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// loadFiles lists the session's worktree for the tree view. The diff view
// needs no listing, so opening it costs no git call.
func (m *Model) loadFiles(id string) tea.Cmd {
	if m.review.View == ViewDiff {
		return nil
	}
	return m.listFiles(id)
}

// relistFiles lists again for a tree that is already loaded, whichever view
// is showing: a turn just ended and the tree behind the diff is as stale as
// the one on screen (#195). A column that never listed has nothing to
// refresh, and pays on its first switch to the tree as before (#131).
func (m *Model) relistFiles(id string) tea.Cmd {
	if m.review.Tree == nil {
		return nil
	}
	return m.listFiles(id)
}

// listFiles runs git off the Update goroutine: on a large tree it takes long
// enough to stall the frame, exactly as for the diff. One listing in flight
// per session, the way pollStat guards its read: two turn ends in quick
// succession must not fork git twice for the same answer (#195).
func (m *Model) listFiles(id string) tea.Cmd {
	sess, ok := m.session(id)
	if !ok || m.filesPending[id] {
		return nil
	}
	m.filesPending[id] = true
	list := m.files
	return func() tea.Msg {
		paths, err := list(sess.Dir)
		return FilesLoadedMsg{SessionID: id, Paths: paths, Err: err}
	}
}

// loadFilesIfMissing lists for a column that switched into the tree without
// ever having listed: the diff-first path, where loadFiles declined because
// the diff was showing and nothing asked again (#131). A listing already in
// memory, or one that failed and is waiting for r, is left alone - the
// optimisation toggleView describes is about not re-forking git for that.
func (m *Model) loadFilesIfMissing(id string) tea.Cmd {
	if m.review.Tree != nil || m.review.TreeErr != "" {
		return nil
	}
	return m.loadFiles(id)
}

// onFilesLoaded builds the tree, unless the column closed or moved to another
// session while git was running. The in-flight guard clears first, whatever
// happened, so a failure never wedges the session's listing.
func (m *Model) onFilesLoaded(msg FilesLoadedMsg) tea.Cmd {
	delete(m.filesPending, msg.SessionID)
	if !m.review.Open || msg.SessionID != m.review.SessionID {
		return nil
	}
	if msg.Err != nil {
		slog.Warn("listing files", "session", msg.SessionID, "err", msg.Err)
		m.review.TreeErr = msg.Err.Error()
		return nil
	}
	m.review.TreeErr = ""
	if m.review.Tree == nil {
		m.review.Tree = review.NewTree(msg.Paths, m.changes())
	} else {
		m.relistUnderCursor(msg.Paths)
	}
	m.contentChanged()
	m.moveTreeCursor(0)
	return nil
}

// relistUnderCursor replaces the listing and keeps the cursor on the path it
// was on: a file claude created above it shifts every row down, and the
// operator reading a file should not find the cursor on its neighbour. A
// path that is gone leaves the cursor at its index, and the clamp that
// follows keeps it on a row (#195).
func (m *Model) relistUnderCursor(paths []string) {
	rows := m.treeRows()
	path := ""
	if m.review.TreeCursor < len(rows) {
		path = rows[m.review.TreeCursor].Path
	}
	m.review.Tree.Relist(paths, m.changes())
	for i, n := range m.treeRows() {
		if n.Path == path {
			m.review.TreeCursor = i
			return
		}
	}
}

// changes is what the loaded diff did to each path, which is the mark and
// the hue beside a row in the tree (#196). The diff and the listing arrive
// independently, so the tree is re-marked whenever either lands. A renamed
// file is keyed on its new name, the one the listing has.
func (m *Model) changes() map[string]review.Change {
	out := map[string]review.Change{}
	for _, f := range m.review.Diff.Files {
		out[f.Path] = review.ChangeOf(f.Status)
	}
	return out
}

// retouchTree re-marks an already-listed tree when a diff lands after it,
// which is the usual order: `git ls-files` returns before `git diff` (#24).
func (m *Model) retouchTree() {
	if m.review.Tree != nil {
		m.review.Tree.Retouch(m.changes())
		m.contentChanged()
	}
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
	switch {
	case n.IsDir:
		m.review.Tree.Toggle(n.Path)
		m.contentChanged()
		m.moveTreeCursor(0)
	case n.Change == review.ChangeDeleted:
		m.previewDeleted(n.Path)
	default:
		m.previewFile(n.Path)
	}
	return nil
}

// previewDeleted opens the preview on a row the diff deleted: there is no
// file to read, and a read error would say "no such file" about a row the
// tree itself put there (#196).
func (m *Model) previewDeleted(rel string) {
	m.review.Preview = review.Preview{Path: rel, Deleted: true}
	m.review.PreviewOffset, m.review.View, m.review.ColOffset = 0, ViewPreview, 0
	m.contentChanged()
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
	stylePreview(&p)
	m.review.Preview, m.review.PreviewOffset, m.review.View = p, 0, ViewPreview
	m.contentChanged()
	m.review.ColOffset = 0 // a new file opens at its left edge, not mid-line (#94)
}

// highlightBudget is the largest file the preview highlights. previewFile
// reads synchronously because 256 KiB is faster than a frame; lexing is
// not: highlight.BenchmarkLines measures chroma's Go lexer at about 40 ms
// on 64 KiB and 170 ms on the full read bound. 40 ms once, when a file is
// opened, is under what a person notices; 170 ms on every large file is a
// stall on the goroutine PTY output queues on (#197).
const highlightBudget = 64 << 10

// stylePreview highlights once, at open, with tabs already expanded so the
// styled line and the plain line previewRow draws agree cell for cell. A
// file with no lexer leaves Styled nil and says nothing; only a file over
// the budget is noted, because that is the one the operator might expect
// coloured.
func stylePreview(p *review.Preview) {
	if p.Binary || p.Deleted || len(p.Lines) == 0 {
		return
	}
	if previewBytes(p.Lines) > highlightBudget {
		p.Unhighlighted = true
		return
	}
	plain := make([]string, len(p.Lines))
	for i, line := range p.Lines {
		plain[i] = expandTabs(line)
	}
	if styled := highlight.Lines(p.Path, plain); !sameLines(styled, plain) {
		p.Styled = styled
	}
}

func previewBytes(lines []string) int {
	n := 0
	for _, l := range lines {
		n += len(l) + 1
	}
	return n
}

func sameLines(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
