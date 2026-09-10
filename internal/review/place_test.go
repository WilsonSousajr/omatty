package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

func commentAt(t *testing.T, d review.Diff, p review.Position, n string) review.Comment {
	t.Helper()
	return review.Comment{Anchor: review.AnchorAt(d, p), Quote: d.LineAt(p).Text, Note: n}
}

func TestPlace_ExactAnchorResolvesToItsLine_issue22(t *testing.T) {
	d := parse(t, twoFileDiff)
	pos := review.Position{File: 0, Hunk: 0, Line: 2}
	c := commentAt(t, d, pos, "why 3")

	p := review.Place(d, []review.Comment{c})

	if got := p.At[pos]; len(got) != 1 || got[0] != 0 {
		t.Errorf("At[%v] = %v, want [0]", pos, got)
	}
	if p.Where[0] != pos {
		t.Errorf("Where[0] = %v, want %v", p.Where[0], pos)
	}
}

// Claude edited lines above the hunk: the header's numbers changed but the
// line is still there, so the comment follows it instead of orphaning.
func TestPlace_FollowsTheLineWhenTheHunkHeaderShifts_issue22(t *testing.T) {
	before := parse(t, twoFileDiff)
	c := commentAt(t, before, review.Position{File: 0, Hunk: 0, Line: 2}, "why 3")
	after := parse(t, strings.Replace(twoFileDiff, "@@ -10,4 +10,5 @@", "@@ -30,4 +31,5 @@", 1))

	p := review.Place(after, []review.Comment{c})

	want := review.Position{File: 0, Hunk: 0, Line: 2}
	if p.Where[0] != want || len(p.Orphans) != 0 {
		t.Errorf("Where[0] = %v, Orphans = %v; want %v and no orphans", p.Where[0], p.Orphans, want)
	}
}

func TestPlace_OrphansWhenTheLineIsGone_issue22(t *testing.T) {
	before := parse(t, twoFileDiff)
	c := commentAt(t, before, review.Position{File: 0, Hunk: 0, Line: 2}, "why 3")
	after := parse(t, strings.Replace(twoFileDiff, "+\tb := 3", "+\tb := 99", 1))

	p := review.Place(after, []review.Comment{c})

	if got := p.Orphans[0]; len(got) != 1 || got[0] != 0 {
		t.Errorf("Orphans[0] = %v, want [0]: the line no longer exists", got)
	}
	if _, ok := p.Where[0]; ok {
		t.Error("an orphan must not have a Where position")
	}
}

func TestPlace_LostWhenTheFileIsGone_issue22(t *testing.T) {
	before := parse(t, twoFileDiff)
	c := commentAt(t, before, review.Position{File: 1, Hunk: 0, Line: 0}, "n")
	after := parse(t, twoFileDiff[:strings.Index(twoFileDiff, "diff --git a/new.txt")])

	p := review.Place(after, []review.Comment{c})

	if len(p.Lost) != 1 || p.Lost[0] != 0 {
		t.Errorf("Lost = %v, want [0]", p.Lost)
	}
}

func TestPlace_RepeatedLinesResolveByOccurrence_issue22(t *testing.T) {
	d := parse(t, dupBraceDiff)
	second := review.Position{File: 0, Hunk: 0, Line: 3}
	c := commentAt(t, d, second, "second brace")

	p := review.Place(d, []review.Comment{c})

	if p.Where[0] != second {
		t.Errorf("Where[0] = %v, want the second brace at %v", p.Where[0], second)
	}
}

// Two notes on one line keep their queue order, so the numbering in the
// composed message matches what the pane shows.
func TestPlace_TwoCommentsOnOneLineKeepTheirOrder_issue22(t *testing.T) {
	d := parse(t, twoFileDiff)
	pos := review.Position{File: 0, Hunk: 0, Line: 2}
	first := commentAt(t, d, pos, "first")
	second := commentAt(t, d, pos, "second")

	p := review.Place(d, []review.Comment{first, second})

	if got := p.At[pos]; len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("At[%v] = %v, want [0 1]", pos, got)
	}
}
