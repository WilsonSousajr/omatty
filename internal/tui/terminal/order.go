package terminal

import (
	"log/slog"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// inputQueue runs one terminal's writes to its child - keys, mouse events,
// SendInput - one at a time, in the order they were queued.
//
// bubbleterm returns every write as its own tea.Cmd, and bubbletea runs
// commands concurrently, so two keys typed in one burst could reach the PTY
// in either order: "word" arrived at codex as "wodr" (#725). Running the
// commands here, on one goroutine, keeps the order the operator typed in.
// Running them inline in Update would keep it too, but a child that stops
// reading would then freeze the whole UI on a full PTY buffer; here only
// this queue waits, and Close unblocks it by closing the PTY.
type inputQueue struct {
	mu      sync.Mutex
	pending []tea.Cmd
	wake    chan struct{}
	stop    chan struct{}
	once    sync.Once
}

// newInputQueue starts a queue and its writer goroutine.
//
//	q := newInputQueue()
//	q.push(cmd) // runs after every command pushed before it
func newInputQueue() *inputQueue {
	q := &inputQueue{wake: make(chan struct{}, 1), stop: make(chan struct{})}
	go q.run()
	return q
}

// push queues cmd behind every write queued before it. A nil cmd - a key
// bubbleterm does not translate - is nothing to write.
func (q *inputQueue) push(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	q.mu.Lock()
	q.pending = append(q.pending, cmd)
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default: // the writer is already due to drain
	}
}

// run drains the queue each time it is woken, until closed.
func (q *inputQueue) run() {
	for {
		select {
		case <-q.stop:
			return
		case <-q.wake:
			q.drain()
		}
	}
}

// drain runs every queued write in order. A write's message is an error or
// nothing; there is no loop to hand it to from here, so an error is logged.
func (q *inputQueue) drain() {
	for cmd := q.take(); cmd != nil; cmd = q.take() {
		if msg := cmd(); msg != nil {
			slog.Debug("writing input to a terminal", "result", msg)
		}
	}
}

// take removes and returns the oldest queued write, or nil.
func (q *inputQueue) take() tea.Cmd {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return nil
	}
	cmd := q.pending[0]
	q.pending = q.pending[1:]
	return cmd
}

// close stops the writer. Writes still queued are dropped: the terminal
// they were for is closing. It is idempotent.
func (q *inputQueue) close() { q.once.Do(func() { close(q.stop) }) }

// isInput reports whether msg is one bubbleterm turns into a write to the
// child, so its command must go through the queue rather than back to
// bubbletea's concurrent loop.
func isInput(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		return true
	}
	return false
}
