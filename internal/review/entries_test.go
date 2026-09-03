package review_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

func kinds(es []review.Entry) []review.EntryKind {
	out := make([]review.EntryKind, len(es))
	for i, e := range es {
		out[i] = e.Kind
	}
	return out
}

func TestFlatten_FileHunkLinesThenCommentsBeneathTheirLine(t *testing.T) {
	d := parse(t, twoFileDiff)
	c := commentAt(t, d, review.Position{File: 0, Hunk: 0, Line: 2}, "why 3")
	p := review.Place(d, []review.Comment{c})

	es := review.Flatten(d, p)

	want := []review.EntryKind{
		review.EntryFile, review.EntryHunk,
		review.EntryLine, review.EntryLine, review.EntryLine, review.EntryComment,
		review.EntryLine, review.EntryLine, review.EntryLine,
		review.EntryFile, review.EntryHunk, review.EntryLine, review.EntryLine,
	}
	got := kinds(es)
	if len(got) != len(want) {
		t.Fatalf("Flatten gave %d entries %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, got[i], want[i])
		}
	}
	if es[5].Comment != 0 || es[5].Pos != (review.Position{File: 0, Hunk: 0, Line: 2}) {
		t.Errorf("comment entry = %+v, want comment 0 on line 2", es[5])
	}
	if es[0].Text != "internal/ui/model.go" || es[1].Text != d.Files[0].Hunks[0].Header {
		t.Errorf("headers = %q, %q", es[0].Text, es[1].Text)
	}
}

func TestFlatten_OrphansFloatToTheTopOfTheirFile_issue22(t *testing.T) {
	d := parse(t, twoFileDiff)
	lost := review.Comment{
		Anchor: review.Anchor{File: "new.txt", Hunk: "@@ gone @@", Hash: "nope"},
		Note:   "moved",
	}
	p := review.Place(d, []review.Comment{lost})

	es := review.Flatten(d, p)

	// The new.txt header is followed directly by its orphan, before the hunk.
	if es[8].Kind != review.EntryFile || es[9].Kind != review.EntryOrphan || es[9].Comment != 0 {
		t.Errorf("entries 8-9 = %+v, %+v; want the new.txt header then its orphan", es[8], es[9])
	}
}

func TestFlatten_AnEmptyDiffHasNoRows(t *testing.T) {
	if es := review.Flatten(review.Diff{}, review.Place(review.Diff{}, nil)); len(es) != 0 {
		t.Errorf("Flatten(empty) = %v, want no rows", es)
	}
}
