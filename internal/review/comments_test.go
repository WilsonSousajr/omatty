package review_test

import (
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/review"
)

func note(n string) review.Comment {
	return review.Comment{Anchor: review.Anchor{File: "f", Hash: n}, Quote: "q", Note: n}
}

func TestComments_QueueInOrderAndRemoveByIndex(t *testing.T) {
	cs := review.NewComments()
	cs.Add(note("one"))
	cs.Add(note("two"))
	cs.Add(note("three"))

	if !cs.Remove(1) {
		t.Fatal("Remove(1) = false, want true")
	}
	if cs.Remove(5) {
		t.Error("Remove(5) = true on a 2-element queue, want false")
	}
	if cs.Remove(-1) {
		t.Error("Remove(-1) = true, want false")
	}
	all := cs.All()
	if cs.Len() != 2 || all[0].Note != "one" || all[1].Note != "three" {
		t.Errorf("after Remove(1): %+v, want [one three]", all)
	}
}

func TestComments_AllIsACopy(t *testing.T) {
	cs := review.NewComments()
	cs.Add(note("one"))

	cs.All()[0].Note = "mutated"

	if cs.All()[0].Note != "one" {
		t.Error("All() exposed the internal slice; a caller mutated the queue")
	}
}

// A sent comment stays queued, so the reviewer can read this turn's diff
// against what they asked for, but it is no longer pending: it is never
// composed again and does not count as waiting to be sent (#335).
func TestComments_MarkSentKeepsThemButNoneArePending_issue335(t *testing.T) {
	at := time.Date(2026, 9, 23, 22, 41, 0, 0, time.UTC)
	cs := review.NewComments()
	cs.Add(note("one"))
	cs.Add(note("two"))

	cs.MarkSent(at)
	cs.Add(note("three"))

	if cs.Len() != 3 {
		t.Errorf("Len() = %d after sending two and adding one, want 3", cs.Len())
	}
	if cs.PendingLen() != 1 {
		t.Errorf("PendingLen() = %d, want 1", cs.PendingLen())
	}
	if p := cs.Pending(); len(p) != 1 || p[0].Note != "three" {
		t.Errorf("Pending() = %+v, want only [three]", p)
	}
	all := cs.All()
	if !all[0].Sent.Equal(at) || !all[1].Sent.Equal(at) {
		t.Errorf("sent comments carry %v and %v, want %v", all[0].Sent, all[1].Sent, at)
	}
	if !all[2].Sent.IsZero() {
		t.Errorf("the comment added after sending is marked sent at %v", all[2].Sent)
	}
}

// MarkSent does not re-date a comment that was already sent: the time it
// shows is when it went, not when the next batch did.
func TestComments_MarkSentKeepsAnEarlierSendTime_issue335(t *testing.T) {
	first := time.Date(2026, 9, 23, 22, 41, 0, 0, time.UTC)
	cs := review.NewComments()
	cs.Add(note("one"))
	cs.MarkSent(first)
	cs.Add(note("two"))

	cs.MarkSent(first.Add(time.Hour))

	if got := cs.All()[0].Sent; !got.Equal(first) {
		t.Errorf("the first comment's send time moved to %v, want %v", got, first)
	}
}

// A sent comment whose line or file is gone has been dealt with one way or
// another; keeping it as "(moved)" would clutter the file with notes about
// code that no longer exists. A pending one is different - it has not been
// sent yet, so it stays and floats up as moved, as it always has (#22).
func TestComments_PruneSentDropsOnlySentCommentsThatNoLongerPlace_issue335(t *testing.T) {
	before := parse(t, twoFileDiff)
	cs := review.NewComments()
	cs.Add(commentAt(t, before, review.Position{File: 0, Hunk: 0, Line: 2}, "sent, line gone"))
	cs.Add(commentAt(t, before, review.Position{File: 0, Hunk: 0, Line: 3}, "sent, line survives"))
	cs.Add(commentAt(t, before, review.Position{File: 1, Hunk: 0, Line: 0}, "sent, file gone"))
	cs.MarkSent(time.Date(2026, 9, 23, 22, 41, 0, 0, time.UTC))
	cs.Add(commentAt(t, before, review.Position{File: 0, Hunk: 0, Line: 2}, "pending, line gone"))

	edited := strings.Replace(twoFileDiff, "+\tb := 3", "+\tb := 99", 1)
	after := parse(t, edited[:strings.Index(edited, "diff --git a/new.txt")])
	cs.PruneSent(after)

	var got []string
	for _, c := range cs.All() {
		got = append(got, c.Note)
	}
	want := []string{"sent, line survives", "pending, line gone"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("after PruneSent: %q, want %q", got, want)
	}
}
