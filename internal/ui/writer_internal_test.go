package ui

import (
	"testing"
	"time"
)

// Writes run in the order they were asked for, one at a time, even when the
// first is slow: a rename asked for second must land second (#653).
func TestWriter_runsWritesInTheOrderAskedOneAtATime_issue653(t *testing.T) {
	var w writer
	var order []string
	release := make(chan struct{})
	first := w.submit(func() error { <-release; order = append(order, "first"); return nil })
	second := w.submit(func() error { order = append(order, "second"); return nil })

	select {
	case <-second:
		t.Fatal("the second write ran while the first was still running")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-first
	<-second
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("writes ran as %v, want [first second]", order)
	}
}
