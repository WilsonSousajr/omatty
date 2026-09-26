// Folding the files nobody wrote out of the review queue (#338).

package ui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// generatedMsg carries one session's detection back into Update.
type generatedMsg struct {
	id  string
	gen map[string]bool
	err error
}

// detectGenerated asks which of the session's listed and changed files nobody
// wrote.
//
// A command rather than a read in Update: the detection asks git about
// .gitattributes and opens Go files to look at their headers, and both belong
// off the goroutine the frame is drawn on - the same reason listFiles is a
// command (#195).
//
// Both sets of paths, because they disagree. A file the session deleted is in
// the diff and gone from the listing, and a lockfile it has not touched is in
// the listing and not in the diff; the queue and the tree each need their own.
func (m *Model) detectGenerated(id string) tea.Cmd {
	sess, ok := m.session(id)
	if !ok {
		return nil
	}
	paths := m.pathsToClassify()
	if len(paths) == 0 {
		return nil
	}
	detect := m.generatedFn
	return func() tea.Msg {
		gen, err := detect(sess, paths)
		return generatedMsg{id: id, gen: gen, err: err}
	}
}

// pathsToClassify is every path the tree lists plus every path the diff
// changes, each once.
func (m *Model) pathsToClassify() []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range m.treeNodes() {
		if !n.IsDir && !seen[n.Path] {
			seen[n.Path] = true
			out = append(out, n.Path)
		}
	}
	for _, f := range m.review.Diff.Files {
		if !seen[f.Path] {
			seen[f.Path] = true
			out = append(out, f.Path)
		}
	}
	return out
}

// treeNodes is every row of the listing, folded or not - Visible would hide
// exactly the files being classified.
func (m *Model) treeNodes() []review.TreeNode {
	if m.review.Tree == nil {
		return nil
	}
	shown := m.review.Tree.GeneratedShown()
	m.review.Tree.ShowGenerated(true)
	defer m.review.Tree.ShowGenerated(shown)
	return m.review.Tree.Visible()
}

// onGenerated stores the detection and folds the tree with it.
//
// A failure leaves the previous answer standing and warns once, the way
// onCoverage does: blanking it would spring every lockfile back into the queue
// on one bad git call, which is noise arriving for no reason the operator can
// see.
func (m *Model) onGenerated(msg generatedMsg) {
	if msg.err != nil {
		slog.Warn("detecting generated files", "session", msg.id, "err", msg.err)
		return
	}
	m.generated[msg.id] = msg.gen
	m.applyGenerated()
}

// applyGenerated hands the shown session's detection to its tree.
func (m *Model) applyGenerated() {
	if m.review.Tree == nil {
		return
	}
	m.review.Tree.SetGenerated(m.generated[m.review.SessionID])
	m.contentChanged()
	m.moveTreeCursor(0)
}

// toggleGenerated is g in the tree: fold the generated files back in, or away
// again. They are away by default - the point of #338 - and this is what keeps
// them reviewable rather than merely gone.
func (m *Model) toggleGenerated() {
	if m.review.Tree == nil {
		return
	}
	m.review.Tree.ShowGenerated(!m.review.Tree.GeneratedShown())
	m.contentChanged()
	m.moveTreeCursor(0)
}

// isGenerated reports whether the shown session's path is one nobody wrote,
// which is what keeps M10's coverage markers off it: a generated file has no
// test and never will, so marking its lines uncovered says something true about
// a file and nothing at all about the change (#338).
func (m *Model) isGenerated(path string) bool {
	return m.generated[m.review.SessionID][path]
}
