package review

import (
	"sort"
	"strings"
)

// TreeNode is one row of the file tree: a directory or a file at a depth.
// Touched means the session changed this file, or a file under this
// directory, which is how a folded directory still says "something in here
// moved" (#24).
type TreeNode struct {
	Path    string
	Name    string
	Depth   int
	IsDir   bool
	Touched bool
}

// Tree is a worktree listing with collapsible directories (#24).
//
//	t := review.NewTree(paths, touched)
//	rows := t.Visible()
type Tree struct {
	nodes     []TreeNode // the full listing in display order
	collapsed map[string]bool
}

// NewTree builds the listing from paths, emitting a directory the first time
// a path passes through it. touched holds the changed file paths. paths are
// sorted here rather than trusted, so a caller that concatenates two git
// listings still gets a directory listing.
func NewTree(paths []string, touched map[string]bool) *Tree {
	t := &Tree{collapsed: map[string]bool{}}
	t.rebuild(paths, touched)
	return t
}

// rebuild replaces the rows from a fresh listing. Sorting and emitting are
// here rather than in NewTree so Relist builds the same shape (#195).
func (t *Tree) rebuild(paths []string, touched map[string]bool) {
	sorted := append([]string(nil), paths...)
	sort.Slice(sorted, func(i, j int) bool { return pathLess(sorted[i], sorted[j]) })
	t.nodes = t.nodes[:0]
	seen := map[string]bool{}
	for _, p := range sorted {
		t.addPath(p, touched, seen)
	}
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
func (t *Tree) addPath(p string, touched, seen map[string]bool) {
	parts := strings.Split(p, "/")
	for i := range parts {
		path := strings.Join(parts[:i+1], "/")
		if seen[path] {
			continue
		}
		seen[path] = true
		isDir := i < len(parts)-1
		t.nodes = append(t.nodes, TreeNode{Path: path, Name: parts[i], Depth: i,
			IsDir: isDir, Touched: touchedUnder(path, isDir, touched)})
	}
}

// touchedUnder reports whether path, or a file beneath it, was changed.
func touchedUnder(path string, isDir bool, touched map[string]bool) bool {
	if !isDir {
		return touched[path]
	}
	for f := range touched {
		if strings.HasPrefix(f, path+"/") {
			return true
		}
	}
	return false
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

// Retouch reapplies the touched set to an existing listing, keeping both the
// shape and the collapse state. The worktree listing and the diff are loaded
// independently and `git ls-files` returns first, so whichever arrives second
// must update the tree rather than rebuild it under the cursor (#24).
//
//	tree.Retouch(map[string]bool{"internal/ui/model.go": true})
func (t *Tree) Retouch(touched map[string]bool) {
	for i, n := range t.nodes {
		t.nodes[i].Touched = touchedUnder(n.Path, n.IsDir, touched)
	}
}

// Relist replaces the rows with a fresh listing and keeps the collapse
// state, the way Retouch keeps it for a fresh diff: a turn ending re-lists
// the worktree so a file claude created appears without r, and a directory
// the operator folded must not spring open under the cursor because of it
// (#195). A folded directory that is no longer listed is forgotten, so a
// later directory of the same name starts open like any other.
//
//	tree.Relist(paths, touched)
func (t *Tree) Relist(paths []string, touched map[string]bool) {
	t.rebuild(paths, touched)
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
