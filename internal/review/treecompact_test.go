package review_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// A chain of directories with one child each is one row: Go trees are deep,
// and at 24 rows the difference between seeing a change and scrolling to it
// is the rows the chain spent (#430, VS Code's compact folders).
func TestVisible_CompactsASingleChildChain_issue430(t *testing.T) {
	tr := review.NewTree([]string{"internal/review/testdata/a.diff", "go.mod"}, nil)

	if got, want := names(tr.Visible()), "internal/review/testdata/| a.diff|go.mod"; got != want {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

// The chain stops where a directory has two children, and resumes below it.
func TestVisible_StopsTheChainAtABranch_issue430(t *testing.T) {
	tr := review.NewTree([]string{"a/b/c/x.go", "a/b/d/y/z.go"}, nil)

	if got, want := names(tr.Visible()), "a/b/| c/|  x.go| d/y/|  z.go"; got != want {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

// A chain ending in a file is not merged into the file: only directories chain.
func TestVisible_AChainEndingInAFileKeepsTheFile_issue430(t *testing.T) {
	tr := review.NewTree([]string{"a/b/only.go"}, nil)

	if got, want := names(tr.Visible()), "a/b/| only.go"; got != want {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

// Folding a compacted row folds the whole chain as one: its path is the
// deepest directory's, so Toggle on it hides everything below.
func TestVisible_FoldingACompactedRowFoldsTheChain_issue430(t *testing.T) {
	tr := review.NewTree([]string{"a/b/c/x.go", "a/b/c/y.go", "top.go"}, nil)
	row := tr.Visible()[0]

	tr.Toggle(row.Path)

	if got, want := names(tr.Visible()), "a/b/c/|top.go"; got != want {
		t.Errorf("after folding %q rows = %q, want %q", row.Path, got, want)
	}
	tr.Toggle(row.Path)
	if got, want := names(tr.Visible()), "a/b/c/| x.go| y.go|top.go"; got != want {
		t.Errorf("after unfolding rows = %q, want %q", got, want)
	}
}

// Under a filter the chain is the matches' ancestors, compacted the same way.
func TestVisible_CompactsUnderAFilter_issue430(t *testing.T) {
	tr := review.NewTree([]string{"a/b/c/x.go", "a/d/y.go"}, nil)
	tr.SetFilter("x.go")

	if got, want := names(tr.Visible()), "a/b/c/| x.go"; got != want {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

// A directory shows the strongest change beneath it - deleted, then added,
// then renamed, then modified - so a folded one says what kind of thing
// happened in it, not only that something did.
func TestVisible_ADirectoryCarriesTheStrongestChangeBeneath_issue430(t *testing.T) {
	changes := map[string]review.Change{
		"d/keep.go": review.ChangeModified, "d/new.go": review.ChangeAdded,
		"e/old.go": review.ChangeDeleted, "e/f.go": review.ChangeAdded,
	}
	tr := review.NewTree([]string{"d/keep.go", "d/new.go", "e/f.go", "x.go"}, changes)

	if got, want := names(tr.Visible()), "d/A| keep.goM| new.goA|e/D| f.goA| old.goD|x.go"; got != want {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

// Changed-only cuts the tree to the files the session touched and the
// directories above them - usually the view wanted mid-session.
func TestVisible_ChangedOnlyKeepsChangedFilesAndTheirParents_issue430(t *testing.T) {
	tr := review.NewTree([]string{"a/x.go", "a/y.go", "b/z.go", "top.go"}, modified("a/y.go", "top.go"))

	tr.SetChangedOnly(true)

	if got, want := names(tr.Visible()), "a/M| y.goM|top.goM"; got != want {
		t.Errorf("rows = %q, want %q", got, want)
	}
	if !tr.ChangedOnly() {
		t.Error("ChangedOnly() = false after SetChangedOnly(true)")
	}
}
