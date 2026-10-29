package terminal

import (
	"log/slog"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// inputQueue runs one terminal's writes to its child - keys and SendInput -
// one at a time, in the order they were queued.
//
// bubbleterm returns every write as its own tea.Cmd, and bubbletea runs
// commands concurrently, so two keys typed in one burst could reach the PTY
// in either order: "word" arrived at codex as "wodr" (#725). Running the
// commands here, on one goroutine, keeps the order the operator typed in.
// Running them inline in Update would keep it too, but a child that stops
// reading would then freeze the whole UI on a full PTY buffer; here only
// this queue waits, and Close unblocks it by closing the PTY.
//
// Off bubbletea's loop, a write has neither its recover nor Guard's, so the
// queue keeps a write's panic for the terminal to re-raise where Guard can
// contain it (invariant 6), and logs a failed write itself (#734).
type inputQueue struct {
	mu      sync.Mutex
	pending []tea.Cmd
	closed  bool
	panic   any // the first panic a write raised; nil until one does
	wake    chan struct{}
	stop    chan struct{}
	exited  chan struct{} // closed when the writer goroutine returns
	once    sync.Once
	log     *slog.Logger
}

// newInputQueue starts a queue and its writer goroutine, reporting through
// log.
//
//	q := newInputQueue(slog.Default())
//	q.push(cmd) // runs after every command pushed before it
func newInputQueue(log *slog.Logger) *inputQueue {
	q := &inputQueue{
		wake: make(chan struct{}, 1), stop: make(chan struct{}), exited: make(chan struct{}), log: log,
	}
	go q.run()
	return q
}

// push queues cmd behind every write queued before it. A nil cmd - a key
// bubbleterm does not translate - is nothing to write. Once the queue is
// closed, or a write has panicked, nothing will run again, so cmd is refused
// rather than left queued forever (#734).
func (q *inputQueue) push(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	q.mu.Lock()
	if q.closed || q.panic != nil {
		q.mu.Unlock()
		return
	}
	q.pending = append(q.pending, cmd)
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default: // the writer is already due to drain
	}
}

// run drains the queue each time it is woken, until closed or until a write
// panics.
func (q *inputQueue) run() {
	defer close(q.exited)
	for {
		select {
		case <-q.stop:
			return
		case <-q.wake:
			if !q.drain() {
				return
			}
		}
	}
}

// drain runs every queued write in order, and reports false once one has
// panicked: the writes behind it are not run.
func (q *inputQueue) drain() bool {
	for batch := q.takeAll(); len(batch) > 0; batch = q.takeAll() {
		for _, cmd := range batch {
			if !q.write(cmd) {
				return false
			}
		}
	}
	return true
}

// write runs one write, containing a panic. Its message is an error or
// nothing, and there is no loop to hand it to from here, so it is logged at
// Warn: at Debug the app's handler dropped it, and a review pasted into a
// dead PTY left no trace (#734).
func (q *inputQueue) write(cmd tea.Cmd) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			q.fail(r)
			ok = false
		}
	}()
	if msg := cmd(); msg != nil {
		q.log.Warn("writing input to a terminal", "result", msg)
	}
	return true
}

// fail keeps a write's panic and drops what is still queued.
func (q *inputQueue) fail(r any) {
	q.mu.Lock()
	q.panic, q.pending = r, nil
	q.mu.Unlock()
	q.log.Error("terminal panicked while writing input", "panic", r)
}

// writePanic is the panic a write raised, or nil. The terminal re-raises it
// on the caller's goroutine, where Guard marks the session crashed.
func (q *inputQueue) writePanic() any {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.panic
}

// takeAll removes and returns every queued write, oldest first, swapping
// the slice out once rather than popping one per lock (#734).
func (q *inputQueue) takeAll() []tea.Cmd {
	q.mu.Lock()
	defer q.mu.Unlock()
	batch := q.pending
	q.pending = nil
	return batch
}

// close stops the writer and refuses later pushes. Writes still queued are
// dropped - the terminal they were for is closing - and the drop is logged
// rather than silent (#734). It is idempotent.
func (q *inputQueue) close() {
	q.once.Do(func() {
		q.mu.Lock()
		q.closed = true
		dropped := len(q.pending)
		q.pending = nil
		q.mu.Unlock()
		if dropped > 0 {
			q.log.Info("dropped input queued for a closing terminal", "writes", dropped)
		}
		close(q.stop)
	})
}

// isInput reports whether msg is one bubbleterm turns into a write to the
// child, so its command must go through the queue rather than back to
// bubbletea's concurrent loop. Only keys: bubbleterm's mouse writes go out
// through the emulator's own goroutine, so queueing them ordered nothing
// (#734).
func isInput(msg tea.Msg) bool {
	_, ok := msg.(tea.KeyMsg)
	return ok
}
