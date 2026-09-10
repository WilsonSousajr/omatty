package ui

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
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
