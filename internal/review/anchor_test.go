package review_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// Invariant 7: an anchor is the line's content, never its number.
func TestAnchorAt_IsFileHunkHeaderAndContentHash_issue22(t *testing.T) {
	d := parse(t, twoFileDiff)

	a := review.AnchorAt(d, review.Position{File: 0, Hunk: 0, Line: 2})

	if a.File != "internal/ui/model.go" || a.Hunk != d.Files[0].Hunks[0].Header {
		t.Errorf("anchor = %+v, want the file path and hunk header", a)
	}
	if a.Hash != review.LineHash(review.Line{Kind: review.LineAdded, Text: "\tb := 3"}) {
		t.Errorf("Hash = %q, want the hash of the added line's kind and text", a.Hash)
	}
	if a.Nth != 0 {
		t.Errorf("Nth = %d, want 0 for the only line with that content", a.Nth)
	}
}

func TestAnchorAt_DistinguishesRepeatedLinesByOccurrence_issue22(t *testing.T) {
	d := parse(t, dupBraceDiff)

	first := review.AnchorAt(d, review.Position{File: 0, Hunk: 0, Line: 0})
	second := review.AnchorAt(d, review.Position{File: 0, Hunk: 0, Line: 3})

	if first.Hash != second.Hash {
		t.Fatalf("two identical lines hashed differently: %q vs %q", first.Hash, second.Hash)
	}
	if first.Nth != 0 || second.Nth != 1 {
		t.Errorf("Nth = %d, %d; want 0 and 1 so the anchors differ", first.Nth, second.Nth)
	}
}

// A line that flips from removed to added is a different line, and a comment
// on the old one must not silently follow the new one.
func TestLineHash_DependsOnKindAsWellAsText(t *testing.T) {
	added := review.LineHash(review.Line{Kind: review.LineAdded, Text: "x"})
	removed := review.LineHash(review.Line{Kind: review.LineRemoved, Text: "x"})

	if added == removed {
		t.Error("a removed and an added line with the same text hashed the same")
	}
	if len(added) != 12 {
		t.Errorf("hash %q is %d chars, want 12 (6 bytes hex)", added, len(added))
	}
}

// The number a line happens to sit on is not part of its identity: that is
// the whole point of invariant 7.
func TestLineHash_IgnoresLineNumbers_issue22(t *testing.T) {
	at10 := review.LineHash(review.Line{Kind: review.LineContext, Text: "}", OldNo: 10, NewNo: 10})
	at99 := review.LineHash(review.Line{Kind: review.LineContext, Text: "}", OldNo: 99, NewNo: 99})

	if at10 != at99 {
		t.Errorf("the same line at two numbers hashed differently: %q vs %q", at10, at99)
	}
}
