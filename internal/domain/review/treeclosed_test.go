package review_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/review"
)

// openAll opens every directory, row by row, until none is left closed. Tests
// written before #593 describe the shape of a fully open tree; this is how
// they still do, through the operator's own key rather than a test-only API.
func openAll(tr *review.Tree) *review.Tree {
	for opened := true; opened; {
		opened = false
		for _, n := range tr.Visible() {
			if n.IsDir && tr.Collapsed(n.Path) {
				tr.Toggle(n.Path)
				opened = true
			}
		}
	}
	return tr
}

// The tree opened with every folder expanded, so on a real repository the
// first screen was the whole listing (#593). Every directory starts closed.
func TestNewTree_everyDirectoryStartsClosed_issue593(t *testing.T) {
	tr := review.NewTree([]string{"a/b.go", "a/c/d.go", "e/f.go", "g.go"}, nil)

	if got := names(tr.Visible()); got != "a/|e/|g.go" {
		t.Errorf("Visible() = %s, want a/|e/|g.go: only the top level", got)
	}
	for _, dir := range []string{"a", "a/c", "e"} {
		if !tr.Collapsed(dir) {
			t.Errorf("Collapsed(%s) = false, want every directory closed", dir)
		}
	}
}

// Opening a directory whose only child is a directory opens the chain, so
// enter shows its contents rather than the same closed row with a longer name
// once per level (#593 with #430's compact rows; VS Code does the same).
func TestTree_openingADirectoryOpensItsSingleChildChain_issue593(t *testing.T) {
	tr := review.NewTree([]string{"a/b/c/d.go", "a/b/c/e.go", "f.go"}, nil)

	tr.Toggle("a")

	if got := names(tr.Visible()); got != "a/b/c/| d.go| e.go|f.go" {
		t.Errorf("Visible() = %s, want the chain open onto its files", got)
	}
}

// A closed directory holding nothing but generated files is still empty: its
// emptiness comes from the full listing, not from the rows a fold hid (#593
// with #338), or every closed coverage/ came back as a row onto nothing.
func TestTree_aClosedDirectoryOfGeneratedFilesIsDropped_issue593(t *testing.T) {
	tr := review.NewTree([]string{"coverage/a.out", "coverage/b.out", "src/x.go"}, nil)
	tr.SetGenerated(map[string]bool{"coverage/a.out": true, "coverage/b.out": true})

	if got := names(tr.Visible()); got != "src/" {
		t.Errorf("Visible() = %s, want src/ alone", got)
	}
}

// A filter and changed-only ignore folds, so a match under a closed directory
// stays reachable (#198, #430), and clearing the filter closes it again.
func TestTree_aMatchUnderAClosedDirectoryIsReachable_issue593(t *testing.T) {
	tr := review.NewTree([]string{"a/b/deep.go", "z.go"}, modified("a/b/deep.go"))

	tr.SetFilter("deep")
	if got := names(tr.Visible()); got != "a/b/M| deep.goM" {
		t.Errorf("filtered = %s, want the match and its ancestors", got)
	}
	tr.SetFilter("")
	tr.SetChangedOnly(true)
	if got := names(tr.Visible()); got != "a/b/M| deep.goM" {
		t.Errorf("changed-only = %s, want the change and its ancestors", got)
	}
	tr.SetChangedOnly(false)
	if got := names(tr.Visible()); got != "a/M|z.go" {
		t.Errorf("cleared = %s, want the fold back", got)
	}
}

// A directory claude creates mid-turn arrives through Relist and starts
// closed like any other, while one the operator opened stays open (#195).
func TestTree_relistKeepsOpenedAndClosesNew_issue593(t *testing.T) {
	tr := review.NewTree([]string{"a/x.go", "b.go"}, nil)
	tr.Toggle("a")

	tr.Relist([]string{"a/x.go", "b.go", "n/new.go"}, nil)

	if got := names(tr.Visible()); got != "a/| x.go|n/|b.go" {
		t.Errorf("Visible() = %s, want a open and n closed", got)
	}
}
