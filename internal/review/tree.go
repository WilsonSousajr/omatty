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
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	t := &Tree{collapsed: map[string]bool{}}
	seen := map[string]bool{}
	for _, p := range sorted {
		t.addPath(p, touched, seen)
	}
	return t
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
	var out []TreeNode
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
