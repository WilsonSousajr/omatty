package terminal

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// A write that fails - a child that has exited, say - answers with an error
// message; the queue logs it and goes on to the writes queued after it, in
// order, and a nil command is nothing to write (#725).
func TestInputQueue_AFailedWriteDoesNotStopTheRest_issue725(t *testing.T) {
	q := newInputQueue()
	defer q.close()
	done := make(chan string, 2)
	q.push(nil)
	q.push(func() tea.Msg { done <- "failed"; return errors.New("write /dev/ptmx: input/output error") })
	q.push(func() tea.Msg { done <- "next"; return nil })

	for _, want := range []string{"failed", "next"} {
		select {
		case got := <-done:
			if got != want {
				t.Fatalf("ran %q, want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%q never ran", want)
		}
	}
	q.close() // idempotent: the deferred close runs too
}
