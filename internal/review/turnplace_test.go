package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// sessionDiff is two turns of work on f.go: a() in the first, b() in the
// second. turnDiff is the second turn alone. Their hunk headers differ - the
// turn's old side counts from its baseline - and "}" is added twice, which is
// what makes a first-occurrence fallback land on the wrong line (#311).
const sessionDiff = `diff --git a/f.go b/f.go
index 1111111..2222222 100644
--- a/f.go
+++ b/f.go
@@ -1 +1,5 @@
 package f
+func a() {
+}
+func b() {
+}
`

const turnDiff = `diff --git a/f.go b/f.go
index 3333333..2222222 100644
--- a/f.go
+++ b/f.go
@@ -1,3 +1,5 @@
 package f
 func a() {
 }
+func b() {
+}
`

// A comment written in the turn view on b's closing brace is about line 5,
// and that is what Claude must be told - not line 3, a()'s brace, which is
// the first "+}" the session diff has.
func TestAnchorFor_aTurnLineAnchorsOnTheSameSessionLine_issue311(t *testing.T) {
	session, turn := parse(t, sessionDiff), parse(t, turnDiff)
	pos := review.Position{File: 0, Hunk: 0, Line: 4} // "+}" of b, new line 5

	c := review.Comment{Anchor: review.AnchorFor(session, turn, pos), Quote: "}", Note: "why"}

	if got := review.Compose(session, []review.Comment{c}); !strings.Contains(got, "f.go:5") {
		t.Errorf("composed %q, want it located at f.go:5", got)
	}
}

// A line the turn shows as context is one the session added: a comment on
// it must still find its line, not be sent as moved.
func TestAnchorFor_aTurnContextLineFindsTheSessionsAddedLine_issue311(t *testing.T) {
	session, turn := parse(t, sessionDiff), parse(t, turnDiff)
	pos := review.Position{File: 0, Hunk: 0, Line: 2} // " }" of a, new line 3

	c := review.Comment{Anchor: review.AnchorFor(session, turn, pos), Quote: "}", Note: "why"}

	if got := review.Compose(session, []review.Comment{c}); !strings.Contains(got, "f.go:3") {
		t.Errorf("composed %q, want it located at f.go:3", got)
	}
}

// The reverse: a comment written in the whole-session view on a()'s brace
// belongs under a()'s brace in the turn view, where it is context - not under
// b()'s "+}", the only "+}" the turn has.
func TestPlaceIn_aSessionCommentLandsOnTheSameLineOfTheTurn_issue311(t *testing.T) {
	session, turn := parse(t, sessionDiff), parse(t, turnDiff)
	c := commentAt(t, session, review.Position{File: 0, Hunk: 0, Line: 2}, "a's brace")

	p := review.PlaceIn(session, turn, []review.Comment{c})

	want := review.Position{File: 0, Hunk: 0, Line: 2}
	if got, ok := p.Where[0]; !ok || got != want {
		t.Errorf("placed at %v (%v), want %v", got, ok, want)
	}
	if len(p.Orphans) != 0 || len(p.Lost) != 0 {
		t.Errorf("orphans %v, lost %v: outside the turn is hidden, never moved", p.Orphans, p.Lost)
	}
}

// A session comment on a line this turn does not show is hidden.
func TestPlaceIn_aSessionCommentOutsideTheTurnIsHidden_issue311(t *testing.T) {
	session := parse(t, sessionDiff)
	turn := parse(t, strings.Replace(turnDiff, "@@ -1,3 +1,5 @@\n package f\n", "@@ -2,2 +2,4 @@\n", 1))
	c := commentAt(t, session, review.Position{File: 0, Hunk: 0, Line: 0}, "package")

	p := review.PlaceIn(session, turn, []review.Comment{c})

	if _, ok := p.Where[0]; ok || len(p.Orphans) != 0 || len(p.Lost) != 0 {
		t.Errorf("a comment outside the turn was placed: %+v", p)
	}
}
