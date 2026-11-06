package forge

import (
	"context"
	"errors"
	"testing"

	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
)

// finishedCounter is a finished-list fetch that counts its calls and fails
// while err is set.
type finishedCounter struct {
	calls int
	err   error
}

func (f *finishedCounter) fetch(context.Context) ([]dforge.PR, error) {
	f.calls++
	return []dforge.PR{{Number: 9, State: dforge.Merged}}, f.err
}

var twoOpen = []dforge.PR{{Number: 1, Head: "a", State: dforge.Open}, {Number: 2, Head: "b", State: dforge.Open}}

// #594: the finished list is only "the work at a worktree's HEAD" (#358), and
// it only gains a change when one leaves the open set. Read on every poll, it
// was a second request a minute per project for nothing.
func TestFinished_ReusedWhileOpenSetIsUnchanged_issue594(t *testing.T) {
	c, f := newFinishedCache(), &finishedCounter{}
	for range 3 {
		got, err := c.read(context.Background(), "p", twoOpen, f.fetch)
		if err != nil || len(got) != 1 {
			t.Fatalf("read = %v, %v, want the one finished change", got, err)
		}
	}
	if f.calls != 1 {
		t.Errorf("fetches = %d, want 1", f.calls)
	}
}

// A new head, or a change leaving the open set, is when the finished list can
// have moved: it is read again at once.
func TestFinished_RereadWhenOpenSetChanges_issue594(t *testing.T) {
	c, f := newFinishedCache(), &finishedCounter{}
	_, _ = c.read(context.Background(), "p", twoOpen, f.fetch)
	pushed := []dforge.PR{{Number: 1, Head: "a2", State: dforge.Open}, twoOpen[1]}
	_, _ = c.read(context.Background(), "p", pushed, f.fetch)
	if f.calls != 2 {
		t.Errorf("fetches after a push = %d, want 2", f.calls)
	}
	_, _ = c.read(context.Background(), "p", pushed[:1], f.fetch)
	if f.calls != 3 {
		t.Errorf("fetches after a change left = %d, want 3", f.calls)
	}
}

// An unchanged project is still read every finishedEvery polls: a change can
// be closed and reopened, or the forge can be wrong once.
func TestFinished_RereadEveryTenPolls_issue594(t *testing.T) {
	c, f := newFinishedCache(), &finishedCounter{}
	for range finishedEvery + 1 {
		_, _ = c.read(context.Background(), "p", twoOpen, f.fetch)
	}
	if f.calls != 2 {
		t.Errorf("fetches over %d reads = %d, want 2", finishedEvery+1, f.calls)
	}
}

// A failed read fails the list, as before, and leaves nothing to reuse.
func TestFinished_ErrorIsNotCached_issue594(t *testing.T) {
	c, f := newFinishedCache(), &finishedCounter{err: errors.New("outage")}
	if _, err := c.read(context.Background(), "p", twoOpen, f.fetch); err == nil {
		t.Fatal("read = nil error, want the outage")
	}
	f.err = nil
	_, _ = c.read(context.Background(), "p", twoOpen, f.fetch)
	if f.calls != 2 {
		t.Errorf("fetches = %d, want 2: the failed read kept nothing", f.calls)
	}
}

// Projects are kept apart by key.
func TestFinished_KeyedByProject_issue594(t *testing.T) {
	c, f := newFinishedCache(), &finishedCounter{}
	_, _ = c.read(context.Background(), "p", twoOpen, f.fetch)
	_, _ = c.read(context.Background(), "q", twoOpen, f.fetch)
	if f.calls != 2 {
		t.Errorf("fetches = %d, want one per project", f.calls)
	}
}
