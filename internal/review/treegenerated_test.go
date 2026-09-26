package review_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// paths returns the visible rows' paths, which is what every assertion here
// is about.
func visiblePaths(t *review.Tree) []string {
	var out []string
	for _, n := range t.Visible() {
		out = append(out, n.Path)
	}
	return out
}

func generatedTree(t *testing.T) *review.Tree {
	t.Helper()
	tree := review.NewTree([]string{
		"go.mod", "go.sum",
		"coverage/lcov.info", "coverage/index.html",
		"internal/ui/model.go", "internal/ui/model.pb.go",
	}, map[string]review.Change{
		"go.sum": review.ChangeModified, "internal/ui/model.go": review.ChangeModified,
	})
	tree.SetGenerated(map[string]bool{
		"go.sum": true, "coverage/lcov.info": true, "coverage/index.html": true,
		"internal/ui/model.pb.go": true,
	})
	return tree
}

// Lockfiles and coverage output sit in the review queue beside source today,
// with equal weight (#338). Folded away by default is the point.
func TestTree_GeneratedFilesAreFoldedAwayByDefault_issue338(t *testing.T) {
	tree := generatedTree(t)

	got := visiblePaths(tree)

	for _, gone := range []string{"go.sum", "coverage/lcov.info", "internal/ui/model.pb.go"} {
		if slicesContains(got, gone) {
			t.Errorf("%s is visible by default; rows = %v", gone, got)
		}
	}
	for _, kept := range []string{"go.mod", "internal/ui/model.go", "internal", "internal/ui"} {
		if !slicesContains(got, kept) {
			t.Errorf("%s went with the generated files; rows = %v", kept, got)
		}
	}
}

// A directory whose every file is generated would otherwise stay as an empty
// row: `coverage/` with nothing under it, which reads as a bug rather than as
// a fold.
func TestTree_ADirectoryOfNothingButGeneratedFilesGoesToo_issue338(t *testing.T) {
	if got := visiblePaths(generatedTree(t)); slicesContains(got, "coverage") {
		t.Errorf("coverage/ is still listed with all of its files hidden; rows = %v", got)
	}
}

// Still listed, still reviewable: one key brings them back, and the same key
// puts them away again.
func TestTree_ShowGeneratedBringsThemBack_issue338(t *testing.T) {
	tree := generatedTree(t)

	tree.ShowGenerated(true)

	got := visiblePaths(tree)
	for _, back := range []string{"go.sum", "coverage", "coverage/lcov.info", "internal/ui/model.pb.go"} {
		if !slicesContains(got, back) {
			t.Errorf("%s did not come back; rows = %v", back, got)
		}
	}

	tree.ShowGenerated(false)
	if slicesContains(visiblePaths(tree), "go.sum") {
		t.Error("go.sum stayed visible after they were put away again")
	}
}

// The count is what the pane title says, so the operator knows something is
// being kept from them rather than guessing.
func TestTree_CountsWhatItIsHiding_issue338(t *testing.T) {
	tree := generatedTree(t)

	if got := tree.GeneratedHidden(); got != 4 {
		t.Errorf("GeneratedHidden() = %d, want 4", got)
	}
	tree.ShowGenerated(true)
	if got := tree.GeneratedHidden(); got != 0 {
		t.Errorf("GeneratedHidden() = %d while they are shown, want 0", got)
	}
}

// A filter is the operator asking for something by name. A file they have
// named is a file they want, whatever this fold thinks of it - the same
// reasoning #198 used for folded directories.
func TestTree_AFilterReachesAGeneratedFile_issue338(t *testing.T) {
	tree := generatedTree(t)

	tree.SetFilter("lcov")

	if got := visiblePaths(tree); !slicesContains(got, "coverage/lcov.info") {
		t.Errorf("a filter naming a generated file did not reach it; rows = %v", got)
	}
}

// Relist keeps the fold, the way it keeps collapse state (#195): a turn
// ending must not spring every lockfile back into the queue.
func TestTree_RelistKeepsTheFold_issue338(t *testing.T) {
	tree := generatedTree(t)

	tree.Relist([]string{"go.mod", "go.sum", "internal/ui/model.go"},
		map[string]review.Change{"go.sum": review.ChangeModified})

	if got := visiblePaths(tree); slicesContains(got, "go.sum") {
		t.Errorf("go.sum came back after a re-list; rows = %v", got)
	}
}

// Regression, found by #24's and #195's own tests while building #338: the fold
// drops a directory nothing visible sits under, and a *collapsed* directory has
// nothing visible under it by definition - Visible already skipped its children.
// The first version therefore deleted any directory the operator folded with
// enter.
func TestTree_ACollapsedDirectoryIsNotMistakenForAnEmptyOne_issue338(t *testing.T) {
	tree := generatedTree(t)

	tree.Toggle("internal")

	got := visiblePaths(tree)
	if !slicesContains(got, "internal") {
		t.Errorf("collapsing internal/ removed its row; rows = %v", got)
	}
	if slicesContains(got, "internal/ui/model.go") {
		t.Errorf("internal/ is collapsed but its children are listed; rows = %v", got)
	}
}

func slicesContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
