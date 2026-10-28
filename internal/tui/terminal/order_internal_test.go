package terminal

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// receive waits for the next write to run and fails unless it is want.
func receive(t *testing.T, ran <-chan string, want string) {
	t.Helper()
	select {
	case got := <-ran:
		if got != want {
			t.Fatalf("ran %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("%q never ran", want)
	}
}

// A write that fails - a child that has exited, say - answers with an error
// message; the queue logs it and goes on to the writes queued after it, in
// order, and a nil command is nothing to write (#725).
func TestInputQueue_AFailedWriteDoesNotStopTheRest_issue725(t *testing.T) {
	q := newInputQueue(slog.New(&recordingHandler{}))
	defer q.close()
	ran := make(chan string, 2)
	q.push(nil)
	q.push(fakeWrite{name: "failed", ran: ran, result: errors.New("write /dev/ptmx: input/output error")}.cmd())
	q.push(fakeWrite{name: "next", ran: ran}.cmd())

	receive(t, ran, "failed")
	receive(t, ran, "next")
	q.close() // idempotent: the deferred close runs too
}

// Regression, issue #734: the error a failed write returns was logged at
// Debug, below the level the app's handler keeps, so a review pasted into a
// dead PTY vanished without a trace. It is a warning.
func TestInputQueue_AFailedWriteIsLoggedAsAWarning_issue734(t *testing.T) {
	logs := &recordingHandler{}
	q := newInputQueue(slog.New(logs))
	defer q.close()
	ran := make(chan string, 2)
	q.push(fakeWrite{name: "failed", ran: ran, result: errors.New("write /dev/ptmx: input/output error")}.cmd())
	q.push(fakeWrite{name: "next", ran: ran}.cmd())
	receive(t, ran, "failed")
	receive(t, ran, "next") // the warning is logged before the next write runs

	if got := logs.at(slog.LevelWarn); len(got) != 1 {
		t.Errorf("%d warnings logged, want 1 for the failed write", len(got))
	}
}

// Regression, issue #734: a write runs on the queue's own goroutine, where
// neither bubbletea's recover nor Guard's can reach, so a panic in the
// emulator's write killed the whole app (invariant 6). The queue contains
// it, writes nothing more, and keeps the panic for the terminal to report.
func TestInputQueue_APanickingWriteIsContained_issue734(t *testing.T) {
	q := newInputQueue(slog.New(&recordingHandler{}))
	defer q.close()
	ran := make(chan string, 3)
	release := make(chan struct{})
	q.push(fakeWrite{name: "held", ran: ran, release: release}.cmd())
	receive(t, ran, "held")
	q.push(fakeWrite{name: "panics", ran: ran, panics: true}.cmd())
	q.push(fakeWrite{name: "after", ran: ran}.cmd())
	close(release)
	receive(t, ran, "panics")

	select {
	case <-q.exited:
	case <-time.After(5 * time.Second):
		t.Fatal("the writer kept running after a write panicked")
	}
	if q.writePanic() == nil {
		t.Error("the panic was not kept for the terminal to report")
	}
	select {
	case got := <-ran:
		t.Errorf("%q ran after a write panicked", got)
	default:
	}
}

// Regression, issue #734: a write's panic is re-raised where Guard can
// contain it - the next render - so the session shows its crash frame
// rather than taking the app down or carrying on as if nothing happened.
func TestBubble_AWritersPanicCrashesThisSessionOnly_issue734(t *testing.T) {
	q := newInputQueue(slog.New(&recordingHandler{}))
	defer q.close()
	ran := make(chan string, 1)
	q.push(fakeWrite{name: "panics", ran: ran, panics: true}.cmd())
	<-q.exited

	g := NewGuard(&bubble{input: q}, "ctrl+o r")
	if frame := g.View(); frame != crashFrameFor("ctrl+o r") || !g.Panicked {
		t.Errorf("View = %q, Panicked = %v; want the crash frame and Panicked", frame, g.Panicked)
	}
}

// Regression, issue #734: a push after close still queued its write, which
// nothing would ever run; writes still queued at close were dropped without
// a word. Now a push after close is refused and a drop is logged.
func TestInputQueue_CloseRefusesPushesAndReportsWhatItDrops_issue734(t *testing.T) {
	logs := &recordingHandler{}
	q := newInputQueue(slog.New(logs))
	ran := make(chan string, 4)
	release := make(chan struct{})
	q.push(fakeWrite{name: "held", ran: ran, release: release}.cmd())
	receive(t, ran, "held")
	q.push(fakeWrite{name: "a", ran: ran}.cmd())
	q.push(fakeWrite{name: "b", ran: ran}.cmd())

	q.close()
	q.push(fakeWrite{name: "late", ran: ran}.cmd())
	close(release)
	<-q.exited

	if n := len(q.pending); n != 0 {
		t.Errorf("%d writes queued after close, want none", n)
	}
	if got := logs.at(slog.LevelInfo); len(got) != 1 {
		t.Errorf("%d info records, want 1 reporting the two dropped writes", len(got))
	}
	if len(ran) != 0 {
		t.Errorf("%q ran after close", <-ran)
	}
}

// Regression, issue #734: bubbleterm's mouse writes go out through the
// emulator's own goroutine, so queueing a mouse event's command ordered
// nothing; only keys are input the queue must run.
func TestIsInput_aMouseEventIsNotQueued_issue734(t *testing.T) {
	if isInput(tea.MouseClickMsg{}) {
		t.Error("isInput(mouse click) = true, want false")
	}
	if !isInput(tea.KeyPressMsg{Code: 'a'}) {
		t.Error("isInput(key) = false, want true")
	}
}
