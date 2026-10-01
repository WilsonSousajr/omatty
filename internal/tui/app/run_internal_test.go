package app

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// Regression, issue #72: Run deferred the listener and tailer closers only,
// so the claude children were left for the OS to reap at exit.
func TestCloseTerminals_ClosesEveryOne_issue72(t *testing.T) {
	a, b := terminal.NewFake(""), terminal.NewFake("")

	CloseTerminals(map[string]terminal.Terminal{"a": a, "b": b})

	if !a.Closed || !b.Closed {
		t.Errorf("closed a=%v b=%v, want both", a.Closed, b.Closed)
	}
}
