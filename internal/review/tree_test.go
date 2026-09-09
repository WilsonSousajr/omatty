package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// names renders the visible rows as "depth-indented name", with / for a
// directory and the change letter (M A D R) for a changed row, so one string
// shows the whole shape. Before #196 the letter was a single *.
func names(nodes []review.TreeNode) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		parts[i] = strings.Repeat(" ", n.Depth) + n.Name
		if n.IsDir {
			parts[i] += "/"
		}
		parts[i] += letter(n.Change)
	}
	return strings.Join(parts, "|")
}

func letter(c review.Change) string {
	return map[review.Change]string{review.ChangeModified: "M", review.ChangeAdded: "A",
		review.ChangeDeleted: "D", review.ChangeRenamed: "R"}[c]
}

// modified is the one-file change map most tests need.
func modified(paths ...string) map[string]review.Change {
	out := map[string]review.Change{}
	for _, p := range paths {
		out[p] = review.ChangeModified
	}
	return out
}

func TestNewTree_PreOrderWithDirectoriesOnFirstSight_issue24(t *testing.T) {
	tr := review.NewTree(
		[]string{"internal/ui/model.go", "internal/ui/render.go", "go.mod", "internal/vcs/git.go"},
		modified("internal/ui/model.go"))

	got := names(tr.Visible())

	// Directories lead their siblings since #194; before that go.mod came
	// first by byte order.
	want := "internal/M| ui/M|  model.goM|  render.go| vcs/|  git.go|go.mod"
	if got != want {
		t.Errorf("Visible() =\n%s\nwant\n%s", got, want)
	}
}

// A file browser lists directories before files at every depth and ignores
// case, so Foo.go and bar.go read as bar, Foo rather than by byte (#194).
func TestNewTree_DirectoriesFirstThenCaseInsensitive_issue194(t *testing.T) {
	tr := review.NewTree([]string{
		"go.mod", "Foo.go", "bar.go", "cmd/main.go",
		"internal/ui/Zed.go", "internal/ui/apple.go", "internal-old/x.go",
	}, nil)

	got := names(tr.Visible())

	want := "cmd/| main.go|internal/| ui/|  apple.go|  Zed.go|internal-old/| x.go|bar.go|Foo.go|go.mod"
	if got != want {
		t.Errorf("Visible() =\n%s\nwant\n%s", got, want)
	}
}

func TestTree_ToggleHidesADirectorysChildren_issue24(t *testing.T) {
	tr := review.NewTree([]string{"a/b.go", "a/c/d.go", "e.go"}, nil)

	tr.Toggle("a")

	if got := names(tr.Visible()); got != "a/|e.go" {
		t.Errorf("after collapsing a: %s, want a/|e.go", got)
	}
	if !tr.Collapsed("a") {
		t.Error("Collapsed(a) = false after Toggle")
	}
	tr.Toggle("a")
	// c/ leads b.go since #194: directories before files at every depth.
	if got := names(tr.Visible()); got != "a/| c/|  d.go| b.go|e.go" {
		t.Errorf("after expanding a again: %s", got)
	}
}

func TestTree_ToggleOnAFileIsIgnored_issue24(t *testing.T) {
	tr := review.NewTree([]string{"e.go"}, nil)
	tr.Toggle("e.go")
	if tr.Collapsed("e.go") || len(tr.Visible()) != 1 {
		t.Error("a file must not collapse")
	}
}

// A directory whose name prefixes a sibling's must not be hidden with it:
// "internal" collapsed hides "internal/ui", never "internal-old".
func TestTree_CollapsingADirectoryLeavesAPrefixSiblingVisible_issue24(t *testing.T) {
	tr := review.NewTree([]string{"a/b.go", "ab/c.go"}, nil)

	tr.Toggle("a")

	if got := names(tr.Visible()); got != "a/|ab/| c.go" {
		t.Errorf("Visible() = %s, want a/|ab/| c.go: ab is not inside a", got)
	}
}

// The listing arrives before the diff, so the marks land on a tree the
// operator may already have folded.
func TestTree_RetouchMarksFilesWithoutLosingTheCollapseState_issue24(t *testing.T) {
	tr := review.NewTree([]string{"a/b.go", "e.go"}, nil)
	tr.Toggle("a")

	tr.Retouch(modified("a/b.go"))

	if got := names(tr.Visible()); got != "a/M|e.go" {
		t.Errorf("Visible() = %s, want a/M|e.go: marked but still folded", got)
	}
	tr.Toggle("a")
	if got := names(tr.Visible()); got != "a/M| b.goM|e.go" {
		t.Errorf("Visible() = %s, want the file marked too", got)
	}
	tr.Retouch(nil)
	if got := names(tr.Visible()); got != "a/| b.go|e.go" {
		t.Errorf("Visible() = %s, want no marks after an empty diff", got)
	}
}

func TestNewTree_NoPathsIsAnEmptyListing_issue24(t *testing.T) {
	if got := review.NewTree(nil, nil).Visible(); len(got) != 0 {
		t.Errorf("Visible() = %v, want none", got)
	}
}

// Three tree states have to stay distinguishable: not requested (nil *Tree),
// loaded and empty, and failed. Visible() returning nil for an empty listing
// collapsed the first two into one and the panel spun forever (#131).
func TestTree_VisibleOnAnEmptyTreeIsEmptyNotNil_issue131(t *testing.T) {
	got := review.NewTree(nil, nil).Visible()
	if got == nil {
		t.Fatal("Visible() on an empty tree = nil, want a non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("Visible() = %v, want no rows", got)
	}
}

// A turn ends and the worktree is listed again; the directory the operator
// folded stays folded and a file claude created appears in place (#195).
func TestTree_RelistKeepsTheCollapseState_issue195(t *testing.T) {
	tr := review.NewTree([]string{"a/b.go", "e.go"}, modified("a/b.go"))
	tr.Toggle("a")

	tr.Relist([]string{"a/b.go", "a/new.go", "e.go"}, modified("a/new.go"))

	if got := names(tr.Visible()); got != "a/M|e.go" {
		t.Errorf("Visible() = %s, want a/M|e.go: still folded, still marked", got)
	}
	tr.Toggle("a")
	if got := names(tr.Visible()); got != "a/M| b.go| new.goM|e.go" {
		t.Errorf("Visible() = %s, want the new file in place and the old mark gone", got)
	}
}

// A folded directory that vanished from the listing must not leave a stale
// entry that folds a future directory of the same name unexpectedly - but a
// directory that is still there keeps its state. Both halves in one test.
func TestTree_RelistForgetsADirectoryThatIsGone_issue195(t *testing.T) {
	tr := review.NewTree([]string{"a/b.go", "c/d.go"}, nil)
	tr.Toggle("a")
	tr.Toggle("c")

	tr.Relist([]string{"c/d.go"}, nil)

	if tr.Collapsed("a") {
		t.Error("Collapsed(a) = true after a was removed from the listing")
	}
	if !tr.Collapsed("c") {
		t.Error("Collapsed(c) = false after a relist that still had c")
	}
}

// The mark says what kind of change: M A D R, coloured the way the diff
// colours those states. A deleted file is in the diff but not in the
// listing, so it becomes a row from the change map; a directory rolls up to
// modified whatever changed beneath it (#196).
func TestTree_RowsCarryTheKindOfChange_issue196(t *testing.T) {
	tr := review.NewTree([]string{"a/m.go", "a/n.go", "r.go"}, map[string]review.Change{
		"a/m.go": review.ChangeModified, "a/n.go": review.ChangeAdded,
		"a/d.go": review.ChangeDeleted, "r.go": review.ChangeRenamed,
	})

	got := names(tr.Visible())

	want := "a/M| d.goD| m.goM| n.goA|r.goR"
	if got != want {
		t.Errorf("Visible() =\n%s\nwant\n%s", got, want)
	}
}

// A deleted file listed by the change map must not survive a relist that no
// longer deletes it, and must not duplicate a path that is also listed.
func TestTree_ADeletedRowFollowsTheChangeMap_issue196(t *testing.T) {
	tr := review.NewTree([]string{"a.go"}, map[string]review.Change{"a.go": review.ChangeDeleted})
	if got := names(tr.Visible()); got != "a.goD" {
		t.Errorf("Visible() = %s, want a.goD: listed and deleted is one row", got)
	}

	tr.Retouch(nil)
	if got := names(tr.Visible()); got != "a.go" {
		t.Errorf("Visible() = %s after Retouch(nil), want a.go", got)
	}
	tr.Relist([]string{"b.go"}, map[string]review.Change{"a.go": review.ChangeDeleted})
	if got := names(tr.Visible()); got != "a.goD|b.go" {
		t.Errorf("Visible() = %s, want the deleted row back beside b.go", got)
	}
}

func TestChangeOf_MapsEveryDiffStatus_issue196(t *testing.T) {
	cases := map[review.FileStatus]review.Change{
		review.FileModified: review.ChangeModified, review.FileAdded: review.ChangeAdded,
		review.FileDeleted: review.ChangeDeleted, review.FileRenamed: review.ChangeRenamed,
	}
	for status, want := range cases {
		if got := review.ChangeOf(status); got != want {
			t.Errorf("ChangeOf(%v) = %v, want %v", status, got, want)
		}
	}
}

// A filter keeps the files whose path fuzzy-matches the query and every
// ancestor of a kept file, so the shape around a match is still readable
// (#198).
func TestTree_SetFilterKeepsMatchesAndTheirAncestors_issue198(t *testing.T) {
	tr := review.NewTree([]string{
		"internal/review/tree.go", "internal/ui/tree.go", "internal/ui/model.go", "go.mod",
	}, nil)

	tr.SetFilter("tree")

	want := "internal/| review/|  tree.go| ui/|  tree.go"
	if got := names(tr.Visible()); got != want {
		t.Errorf("Visible() =\n%s\nwant\n%s", got, want)
	}
}

// A match under a folded directory is reachable: directories read as
// expanded while a filter is on, and the fold comes back when it clears.
func TestTree_AFilterOpensFoldedDirectoriesAndClearingRestoresThem_issue198(t *testing.T) {
	tr := review.NewTree([]string{"a/b.go", "a/c.go", "d.go"}, nil)
	tr.Toggle("a")

	tr.SetFilter("b")
	if got := names(tr.Visible()); got != "a/| b.go" {
		t.Errorf("Visible() = %s while filtering, want a/| b.go: the fold must not hide the match", got)
	}

	tr.SetFilter("")
	if got := names(tr.Visible()); got != "a/|d.go" {
		t.Errorf("Visible() = %s after clearing, want a/|d.go: the fold must come back", got)
	}
}

// A query nothing matches is an empty, non-nil listing, the way an empty
// repository is (#131): the ui tells "nothing matched" from "not listed".
func TestTree_AFilterNothingMatchesIsEmptyNotNil_issue198(t *testing.T) {
	tr := review.NewTree([]string{"a.go"}, nil)
	tr.SetFilter("zzz")
	got := tr.Visible()
	if got == nil || len(got) != 0 {
		t.Errorf("Visible() = %v, want a non-nil empty slice", got)
	}
	if tr.Filter() != "zzz" {
		t.Errorf("Filter() = %q, want the query back", tr.Filter())
	}
}
