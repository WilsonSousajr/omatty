package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

func TestCompose_OneNumberedItemPerCommentWithFileLineQuoteAndNote_issue23(t *testing.T) {
	d := parse(t, twoFileDiff)
	cs := []review.Comment{
		commentAt(t, d, review.Position{File: 0, Hunk: 0, Line: 2}, "use a match here"),
		commentAt(t, d, review.Position{File: 0, Hunk: 0, Line: 1}, "was this needed?"),
		commentAt(t, d, review.Position{File: 1, Hunk: 0, Line: 0}, "name this file"),
	}

	got := review.Compose(d, cs)

	want := "Review comments (3):\n" +
		"\n1. internal/ui/model.go:11\n   > \tb := 3\n   use a match here\n" +
		"\n2. internal/ui/model.go:11\n   > \tb := 2\n   was this needed?\n" +
		"\n3. new.txt:1\n   > fresh\n   name this file"
	if got != want {
		t.Errorf("Compose() =\n%q\nwant\n%q", got, want)
	}
}

// An orphan has no line at all, so it names the file and says why.
func TestCompose_OrphanSaysTheLineMoved_issue23(t *testing.T) {
	d := parse(t, twoFileDiff)
	orphan := review.Comment{
		Anchor: review.Anchor{File: "new.txt", Hash: "gone"},
		Quote:  "old text",
		Note:   "still relevant",
	}

	got := review.Compose(d, []review.Comment{orphan})

	if !strings.Contains(got, "1. new.txt (line moved or removed)\n   > old text\n   still relevant") {
		t.Errorf("Compose(orphan) =\n%s", got)
	}
}

// A comment on a file that vanished still travels: the operator wrote it
// about something, and silently dropping it is worse than citing a path.
func TestCompose_LostCommentStillNamesItsFile_issue23(t *testing.T) {
	d := parse(t, twoFileDiff)
	lost := review.Comment{
		Anchor: review.Anchor{File: "deleted.go", Hash: "gone"},
		Quote:  "x := 1",
		Note:   "check this",
	}

	got := review.Compose(d, []review.Comment{lost})

	if !strings.Contains(got, "1. deleted.go (line moved or removed)") {
		t.Errorf("Compose(lost) =\n%s", got)
	}
}

func TestCompose_NeverEndsWithANewline(t *testing.T) {
	d := parse(t, twoFileDiff)

	got := review.Compose(d, []review.Comment{commentAt(t, d, review.Position{}, "x")})

	if strings.HasSuffix(got, "\n") {
		t.Error("body ends with a newline; the envelope adds the one carriage return that submits")
	}
}
