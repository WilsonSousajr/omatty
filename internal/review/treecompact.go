// Compact folders and changed-only (#430).
//
// A Go tree is deep: internal/review/testdata is three rows before anything in
// it, and at 24 rows those are rows the change itself does not get. A chain of
// directories with one child each is drawn as one row - VS Code's compact
// folders, neo-tree's group_empty_dirs. neo-tree has shipped regressions in
// exactly this (neo-tree #2099), which is why the edge cases each have a test.

package review

import (
	"strings"

	"github.com/WilsonSousajr/omatty/internal/fuzzy"
)

// changeStrength ranks what a directory's letter says: the strongest change
// beneath it. Deleted outranks added because a deletion is the one change
// whose file is no longer there to look at unless the row says so.
var changeStrength = map[Change]int{
	ChangeModified: 1, ChangeRenamed: 2, ChangeAdded: 3, ChangeDeleted: 4,
}

// SetChangedOnly narrows Visible to the files the session changed and the
// directories above them, or lifts the narrowing.
//
//	tree.SetChangedOnly(true)
func (t *Tree) SetChangedOnly(on bool) { t.changedOnly = on }

// ChangedOnly reports whether the listing is narrowed to changed files.
func (t *Tree) ChangedOnly() bool { return t.changedOnly }

// selected is whether a file survives the filter and changed-only together.
// Directories are never selected themselves; they come with what is under them.
func (t *Tree) selected(n TreeNode) bool {
	if n.IsDir || (t.changedOnly && n.Change == ChangeNone) {
		return false
	}
	if t.filter == "" {
		return true
	}
	_, ok := fuzzy.Match(t.filter, n.Path)
	return ok
}

// compact draws every single-child directory chain as one row. The row takes
// the deepest directory's path, so folding it folds the chain as one and the
// fold survives the next listing; its name is the chain's, "a/b/c"; and every
// row under it moves up by the levels the chain saved. rows is pre-order, so a
// merged directory is always the row right after the one it joins.
func compact(rows []TreeNode) []TreeNode {
	merged := mergedDirs(rows)
	out := make([]TreeNode, 0, len(rows))
	for _, n := range rows {
		if merged[n.Path] {
			last := &out[len(out)-1]
			last.Name, last.Path, last.Change = last.Name+"/"+n.Name, n.Path, n.Change
			continue
		}
		n.Depth -= mergedAbove(merged, n.Path)
		out = append(out, n)
	}
	return out
}

// mergedDirs is the directories that join their parent's row: the only child
// of a directory, and a directory itself. A folded directory's children are
// not in rows, so it has none and nothing joins below it.
func mergedDirs(rows []TreeNode) map[string]bool {
	merged := map[string]bool{}
	for i, n := range rows {
		if !n.IsDir {
			continue
		}
		if child, only := onlyChild(rows, i); only && child.IsDir {
			merged[child.Path] = true
		}
	}
	return merged
}

// onlyChild is rows[i]'s single direct child, and whether it has exactly one.
func onlyChild(rows []TreeNode, i int) (TreeNode, bool) {
	prefix, count := rows[i].Path+"/", 0
	var child TreeNode
	for j := i + 1; j < len(rows) && strings.HasPrefix(rows[j].Path, prefix); j++ {
		if rows[j].Depth == rows[i].Depth+1 {
			child, count = rows[j], count+1
		}
	}
	return child, count == 1
}

// mergedAbove counts the merged directories on path's way up, which is how
// many levels its row moves up.
func mergedAbove(merged map[string]bool, path string) int {
	n := 0
	for i := strings.LastIndex(path, "/"); i > 0; i = strings.LastIndex(path, "/") {
		path = path[:i]
		if merged[path] {
			n++
		}
	}
	return n
}
