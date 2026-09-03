package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// names renders the visible rows as "depth-indented name", with / for a
// directory and * for touched, so one string shows the whole shape.
func names(nodes []review.TreeNode) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		parts[i] = strings.Repeat(" ", n.Depth) + n.Name
		if n.IsDir {
			parts[i] += "/"
		}
		if n.Touched {
			parts[i] += "*"
		}
	}
	return strings.Join(parts, "|")
}

func TestNewTree_PreOrderWithDirectoriesOnFirstSight_issue24(t *testing.T) {
	tr := review.NewTree(
		[]string{"internal/ui/model.go", "internal/ui/render.go", "go.mod", "internal/vcs/git.go"},
		map[string]bool{"internal/ui/model.go": true})

	got := names(tr.Visible())

	want := "go.mod|internal/*| ui/*|  model.go*|  render.go| vcs/|  git.go"
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
	if got := names(tr.Visible()); got != "a/| b.go| c/|  d.go|e.go" {
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

func TestNewTree_NoPathsIsAnEmptyListing_issue24(t *testing.T) {
	if got := review.NewTree(nil, nil).Visible(); len(got) != 0 {
		t.Errorf("Visible() = %v, want none", got)
	}
}
