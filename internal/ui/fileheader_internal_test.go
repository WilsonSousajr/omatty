package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// scratchFile is the file #291 was found on: a path deep enough to have
// something to give up, thirteen added lines and five of them uncovered.
func scratchFile() review.File {
	return review.File{Path: "internal/paths/scratch.go", Hunks: addedHunks(13, 0)}
}

// addedHunks is one hunk of a added and r removed lines, so File.Counts reports
// the diffstat the header prints.
func addedHunks(a, r int) []review.Hunk {
	lines := make([]review.Line, 0, a+r)
	for range a {
		lines = append(lines, review.Line{Kind: review.LineAdded})
	}
	for range r {
		lines = append(lines, review.Line{Kind: review.LineRemoved})
	}
	return []review.Hunk{{Lines: lines}}
}

// The header gives its parts up in the stated order at the column widths the
// layout actually produces: 51 cells at a 160-column window, then 43, 35, 27
// and 23 at the default 80 (#291).
func TestFileHeading_givesUpItsPartsInOrder_issue291(t *testing.T) {
	const note = "  5 uncovered"
	cases := []struct {
		budget int
		want   string
	}{
		{51, "internal/paths/scratch.go +13 -0  5 uncovered"},
		{43, "…/paths/scratch.go +13 -0  5 uncovered"},
		{35, "…/scratch.go +13 -0  5 uncovered"},
		{27, "…/scratch.go  5 uncovered"},
		{23, "scratch.go  5 uncovered"},
	}
	for _, c := range cases {
		if got := fileHeading(scratchFile(), note, c.budget); got != c.want {
			t.Errorf("fileHeading at %d cells = %q, want %q", c.budget, got, c.want)
		}
	}
}

// The count is never the part given up, at any width, and the drawn header
// never runs past its column.
func TestFileHeading_keepsTheCountAndTheBudget_issue291(t *testing.T) {
	const note = "  5 uncovered"
	for budget := 23; budget <= 60; budget++ {
		got := fileHeading(scratchFile(), note, budget)
		if !strings.Contains(got, note) {
			t.Errorf("at %d cells the header is %q, want the count whole", budget, got)
		}
		if w := lipgloss.Width(got); w > budget {
			t.Errorf("at %d cells the header is %d wide: %q", budget, w, got)
		}
	}
}

// With no overlay loaded there is no count, and the diffstat survives further
// down because there is more room for it.
func TestFileHeading_withNoCountKeepsTheDiffstat_issue291(t *testing.T) {
	if got, want := fileHeading(scratchFile(), "", 23), "…/scratch.go +13 -0"; got != want {
		t.Errorf("fileHeading = %q, want %q", got, want)
	}
}

// A file at the repository root has no directory to give up, so the diffstat
// is the only part that can go.
func TestFileHeading_aRootFileHasNoDirectoryToGiveUp_issue291(t *testing.T) {
	f := review.File{Path: "go.mod", Hunks: addedHunks(1, 0)}
	if got, want := fileHeading(f, "  1 uncovered", 23), "go.mod  1 uncovered"; got != want {
		t.Errorf("fileHeading = %q, want %q", got, want)
	}
}

// A rename holds two paths. The new name keeps its filename; the old one
// shortens first and gives up to a bare "…", which still says the file came
// from somewhere.
func TestFileHeading_aRenameGivesUpTheOldNameFirst_issue291(t *testing.T) {
	f := review.File{
		Path:    "internal/ui/fileheader.go",
		OldPath: "internal/ui/reviewview.go",
		Status:  review.FileRenamed,
		Hunks:   addedHunks(1, 1),
	}
	cases := []struct {
		budget int
		want   string
	}{
		{60, "internal/ui/reviewview.go → internal/ui/fileheader.go +1 -1"},
		{45, "…/ui/reviewview.go → …/ui/fileheader.go +1 -1"},
		{35, "reviewview.go → fileheader.go +1 -1"},
		{23, "… → fileheader.go +1 -1"},
	}
	for _, c := range cases {
		if got := fileHeading(f, "", c.budget); got != c.want {
			t.Errorf("a rename at %d cells = %q, want %q", c.budget, got, c.want)
		}
	}
}

// A binary has no counts at all; "(binary)" takes the diffstat's place and is
// never what a narrow column gives up.
func TestFileHeading_aBinaryKeepsItsNote_issue291(t *testing.T) {
	f := review.File{Path: "testdata/recorded/session.ansi", Binary: true}
	for budget := 20; budget <= 45; budget++ {
		got := fileHeading(f, "", budget)
		if !strings.Contains(got, binaryNote) {
			t.Errorf("at %d cells the header is %q, want the binary note", budget, got)
		}
		if w := lipgloss.Width(got); w > budget {
			t.Errorf("at %d cells the header is %d wide: %q", budget, w, got)
		}
	}
}
