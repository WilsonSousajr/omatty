package review

import (
	"sort"
	"strings"
)

// TreeNode is one row of the file tree: a directory or a file at a depth.
// Change says what the session did to this file; a directory holding a
// changed file reads as modified, which is how a folded directory still says
// "something in here moved" (#24, #196).
type TreeNode struct {
	Path   string
	Name   string
	Depth  int
	IsDir  bool
	Change Change
}

// Tree is a worktree listing with collapsible directories (#24).
//
//	t := review.NewTree(paths, changes)
//	rows := t.Visible()
type Tree struct {
	nodes     []TreeNode // the full listing in display order
	collapsed map[string]bool
}

// NewTree builds the listing from paths, emitting a directory the first time
// a path passes through it. changes holds what the session did to each
// changed file. paths are sorted here rather than trusted, so a caller that
// concatenates two git listings still gets a directory listing.
func NewTree(paths []string, changes map[string]Change) *Tree {
	t := &Tree{collapsed: map[string]bool{}}
	t.rebuild(paths, changes)
	return t
}

// rebuild replaces the rows from a fresh listing. Sorting and emitting are
// here rather than in NewTree so Relist builds the same shape (#195).
func (t *Tree) rebuild(paths []string, changes map[string]Change) {
	sorted := withDeleted(paths, changes)
	sort.Slice(sorted, func(i, j int) bool { return pathLess(sorted[i], sorted[j]) })
	t.nodes = t.nodes[:0]
	seen := map[string]bool{}
	for _, p := range sorted {
		t.addPath(p, changes, seen)
	}
}

// withDeleted appends the deleted files to a copy of the listing. A deleted
// file is in the diff but gone from `git ls-files`, so without this the tree
// never showed what went away (#196). A path both listed and deleted (a
// file recreated untracked, say) is not doubled: addPath skips seen paths.
func withDeleted(paths []string, changes map[string]Change) []string {
	out := append([]string(nil), paths...)
	for p, c := range changes {
		if c == ChangeDeleted {
			out = append(out, p)
		}
	}
	return out
}

// pathLess orders a listing the way a file browser reads it: at each depth a
// directory before a file, then names without regard to case. Comparing one
// component at a time is what keeps a directory's subtree one contiguous run,
// which Visible relies on to fold it; a plain sort.Strings put go.mod above
// internal/ and Foo.go above bar.go (#194).
func pathLess(a, b string) bool {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := compareComponent(as, bs, i); c != 0 {
			return c < 0
		}
	}
	return len(as) < len(bs)
}

// compareComponent orders the i-th components of two split paths: the one
// with more components after it is a directory and comes first; then the
// case-folded names; then the bytes, so two names that fold alike still have
// one order.
func compareComponent(as, bs []string, i int) int {
	aDir, bDir := i < len(as)-1, i < len(bs)-1
	if aDir != bDir {
		if aDir {
			return -1
		}
		return 1
	}
	if c := strings.Compare(strings.ToLower(as[i]), strings.ToLower(bs[i])); c != 0 {
		return c
	}
	return strings.Compare(as[i], bs[i])
}

// addPath emits every ancestor of p that has not been emitted yet, then p.
func (t *Tree) addPath(p string, changes map[string]Change, seen map[string]bool) {
	parts := strings.Split(p, "/")
	for i := range parts {
		path := strings.Join(parts[:i+1], "/")
		if seen[path] {
			continue
		}
		seen[path] = true
		isDir := i < len(parts)-1
		t.nodes = append(t.nodes, TreeNode{Path: path, Name: parts[i], Depth: i,
			IsDir: isDir, Change: changeUnder(path, isDir, changes)})
	}
}

// changeUnder is the change at path: a file's own, or modified for a
// directory with any change beneath it. A directory is never added or
// deleted here, even when every file in it was: git tracks files, not
// directories, and the row is a listing artefact rather than a change.
func changeUnder(path string, isDir bool, changes map[string]Change) Change {
	if !isDir {
		return changes[path]
	}
	for f := range changes {
		if strings.HasPrefix(f, path+"/") {
			return ChangeModified
		}
	}
	return ChangeNone
}

// Visible returns the rows with collapsed directories' children skipped. The
// listing is pre-order, so a collapsed directory's subtree is the contiguous
// run of nodes under its path; the trailing slash is what keeps "internal"
// from swallowing a sibling named "internal-old".
func (t *Tree) Visible() []TreeNode {
	// Non-nil even when empty: the ui tells "not listed yet" from "listed,
	// nothing there" by this (#131).
	out := make([]TreeNode, 0, len(t.nodes))
	hidden := ""
	for _, n := range t.nodes {
		if hidden != "" && strings.HasPrefix(n.Path, hidden) {
			continue
		}
		hidden = ""
		out = append(out, n)
		if n.IsDir && t.collapsed[n.Path] {
			hidden = n.Path + "/"
		}
	}
	return out
}

// Retouch reapplies the changes to an existing listing, keeping both the
// shape and the collapse state. The worktree listing and the diff are loaded
// independently and `git ls-files` returns first, so whichever arrives second
// must update the tree rather than rebuild it under the cursor (#24). A row
// that only existed because it was deleted stays until the next Relist: the
// shape is the listing's to change, not the diff's.
//
//	tree.Retouch(map[string]review.Change{"internal/ui/model.go": review.ChangeModified})
func (t *Tree) Retouch(changes map[string]Change) {
	for i, n := range t.nodes {
		t.nodes[i].Change = changeUnder(n.Path, n.IsDir, changes)
	}
}

// Relist replaces the rows with a fresh listing and keeps the collapse
// state, the way Retouch keeps it for a fresh diff: a turn ending re-lists
// the worktree so a file claude created appears without r, and a directory
// the operator folded must not spring open under the cursor because of it
// (#195). A folded directory that is no longer listed is forgotten, so a
// later directory of the same name starts open like any other.
//
//	tree.Relist(paths, changes)
func (t *Tree) Relist(paths []string, changes map[string]Change) {
	t.rebuild(paths, changes)
	present := map[string]bool{}
	for _, n := range t.nodes {
		present[n.Path] = n.IsDir
	}
	for dir := range t.collapsed {
		if !present[dir] {
			delete(t.collapsed, dir)
		}
	}
}

// Toggle collapses or expands the directory at path; files are ignored, so
// enter on a file is free to mean something else.
func (t *Tree) Toggle(path string) {
	for _, n := range t.nodes {
		if n.Path == path && n.IsDir {
			t.collapsed[path] = !t.collapsed[path]
			return
		}
	}
}

// Collapsed reports whether the directory at path is collapsed.
func (t *Tree) Collapsed(path string) bool { return t.collapsed[path] }
