package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// fragmentDiff is one file, one hunk, with a line worth commenting on part of.
const fragmentDiff = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -10,3 +10,3 @@ func f() {
 	a := 1
-	b := 2
+	if err := do(ctx, timeout); err != nil { return err }
 }
`

func fragmentParsed(t *testing.T) review.Diff {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader(fragmentDiff))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// #339's second half: a note about part of a line says which part, so claude
// is told "this call" rather than "this line".
func TestCompose_AFragmentIsWhatTheNoteIsAbout_issue339(t *testing.T) {
	d := fragmentParsed(t)
	at := review.AnchorAt(d, review.Position{File: 0, Hunk: 0, Line: 2})

	body := review.Compose(d, []review.Comment{{
		Anchor:   at,
		Quote:    "	if err := do(ctx, timeout); err != nil { return err }",
		Fragment: "do(ctx, timeout)",
		Note:     "the timeout is the wrong unit",
	}})

	if !strings.Contains(body, "do(ctx, timeout)") {
		t.Errorf("the fragment is not in the composed message:\n%s", body)
	}
	if !strings.Contains(body, "a.go:11") {
		t.Errorf("the line reference went missing:\n%s", body)
	}
	if !strings.Contains(body, "the timeout is the wrong unit") {
		t.Errorf("the note went missing:\n%s", body)
	}
}

// A comment with no fragment must compose exactly as it did before #339: the
// whole line, quoted, and nothing extra.
func TestCompose_NoFragmentIsUnchanged_issue339(t *testing.T) {
	d := fragmentParsed(t)
	at := review.AnchorAt(d, review.Position{File: 0, Hunk: 0, Line: 2})
	c := review.Comment{Anchor: at, Quote: "	if err := do(ctx, timeout); err != nil { return err }", Note: "why?"}

	body := review.Compose(d, []review.Comment{c})

	if strings.Contains(body, "about") {
		t.Errorf("a whole-line comment gained a fragment note:\n%s", body)
	}
	if want := "> 	if err := do(ctx, timeout); err != nil { return err }"; !strings.Contains(body, want) {
		t.Errorf("the quoted line changed shape:\n%s", body)
	}
}

// The question #339 requires an answer to: the line survives, the fragment
// does not. The note still travels - the operator wrote it about something -
// and it says the fragment is gone rather than quoting text that is no longer
// there as though it were.
func TestCompose_AFragmentThatIsNoLongerInTheLineSaysSo_issue339(t *testing.T) {
	d := fragmentParsed(t)
	at := review.AnchorAt(d, review.Position{File: 0, Hunk: 0, Line: 2})

	body := review.Compose(d, []review.Comment{{
		Anchor:   at,
		Quote:    "	if err := do(ctx, timeout); err != nil { return err }",
		Fragment: "do(ctx, deadline)", // never in that line
		Note:     "the timeout is the wrong unit",
	}})

	if !strings.Contains(body, "the timeout is the wrong unit") {
		t.Errorf("the note was dropped with its fragment:\n%s", body)
	}
	if !strings.Contains(body, "no longer") {
		t.Errorf("the composed message does not say the fragment is gone:\n%s", body)
	}
}

// Invariant 7, stated as a test: a fragment is a property of the note, not of
// the line's identity. Putting it in the Anchor would stop resolve() matching,
// so two comments on one line with different fragments must anchor identically
// and both place.
func TestPlace_AFragmentIsNotPartOfTheAnchor_issue339(t *testing.T) {
	d := fragmentParsed(t)
	pos := review.Position{File: 0, Hunk: 0, Line: 2}
	at := review.AnchorAt(d, pos)
	first := review.Comment{Anchor: at, Fragment: "do(ctx, timeout)", Note: "unit"}
	second := review.Comment{Anchor: at, Fragment: "return err", Note: "wrap it"}

	p := review.Place(d, []review.Comment{first, second})

	if got := p.At[pos]; len(got) != 2 {
		t.Fatalf("At[%v] = %v, want both comments on the line", pos, got)
	}
	if _, ok := p.Where[0]; !ok {
		t.Error("the first comment did not place")
	}
	if _, ok := p.Where[1]; !ok {
		t.Error("the second comment did not place")
	}
}

// Several comments on one line, each numbered and each sent - the first half
// of #339, which the anchor already supported (`Nth` disambiguates repeated
// lines) and nothing had asserted end to end.
func TestCompose_EveryCommentOnOneLineIsSent_issue339(t *testing.T) {
	d := fragmentParsed(t)
	at := review.AnchorAt(d, review.Position{File: 0, Hunk: 0, Line: 2})

	body := review.Compose(d, []review.Comment{
		{Anchor: at, Quote: "x", Note: "first thing"},
		{Anchor: at, Quote: "x", Note: "second thing"},
	})

	if !strings.Contains(body, "(2)") {
		t.Errorf("the count does not say two:\n%s", body)
	}
	for _, want := range []string{"1. ", "2. ", "first thing", "second thing"} {
		if !strings.Contains(body, want) {
			t.Errorf("%q missing from:\n%s", want, body)
		}
	}
}
