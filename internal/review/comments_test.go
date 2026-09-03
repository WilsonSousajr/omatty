package review_test

import (
	"testing"

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

func TestComments_ClearEmptiesTheQueue(t *testing.T) {
	cs := review.NewComments()
	cs.Add(note("one"))

	cs.Clear()

	if cs.Len() != 0 {
		t.Errorf("Len() = %d after Clear, want 0", cs.Len())
	}
}
