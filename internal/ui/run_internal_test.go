package ui

import (
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Regression, issue #72: Run deferred the listener and tailer closers only,
// so the claude children were left for the OS to reap at exit.
func TestCloseTerminals_ClosesEveryOne_issue72(t *testing.T) {
	a, b := termwrap.NewFake(""), termwrap.NewFake("")

	closeTerminals(map[string]termwrap.Terminal{"a": a, "b": b})

	if !a.Closed || !b.Closed {
		t.Errorf("closed a=%v b=%v, want both", a.Closed, b.Closed)
	}
}

// Replaces TestWireStatus_StartsATailerPerSessionAndClosesThemAll_issue19,
// which asserted only that a model came back (issue #80). One tailer per
// registered session, one more per session created at runtime, and Close
// stops every one of them.
func TestWireStatus_OneTailerPerSessionPlusRuntimeOnes_issue19(t *testing.T) {
	st := registry.State{
		Projects: []registry.Project{{Name: "p", Root: "/p"}},
		Sessions: []registry.Session{{ID: "s1", Project: "p", Title: "one"}, {ID: "s2", Project: "p", Title: "two"}},
	}
	create := func(_, title, _ string) (registry.Session, error) {
		return registry.Session{ID: "s3", Project: "p", Title: title}, nil
	}
	start := func(registry.Session, int, int) (termwrap.Terminal, error) { return termwrap.NewFake(""), nil }
	m, tailers, closeTailers := wireStatus(t.TempDir(), st, map[string]termwrap.Terminal{}, create, start,
		make(chan watcher.Event, 8))

	if len(*tailers) != 2 {
		t.Fatalf("%d tailers for 2 sessions, want 2", len(*tailers))
	}
	if _, err := m.addSession("p", "three", ""); err != nil {
		t.Fatal(err)
	}
	if len(*tailers) != 3 {
		t.Errorf("%d tailers after a runtime session, want 3", len(*tailers))
	}
	closeTailers()
	for _, tl := range *tailers {
		select {
		case <-tl.Done():
		case <-time.After(2 * time.Second):
			t.Error("a tailer is still running after closeTailers")
		}
	}
}
