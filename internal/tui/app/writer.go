package app

import "sync"

// writer runs the registry writes the TUI asks for one at a time, in the order
// Update asked for them (migration step 5.6b, #653).
//
// While every write ran inside Update the order was the order of the keys and
// no two could overlap. Off the Update goroutine each command runs in its own
// goroutine, and every write is a load, an edit and a save of one state.json:
// two at once lose one of the edits, and two renames can land in either
// order. So writes are queued here when Update asks for them, and a single
// worker runs them in that order. A write runs whether or not anything waits
// for its answer, as it did when it ran inside Update.
type writer struct {
	mu      sync.Mutex
	queue   []func()
	waiting *sync.Cond
	started bool
}

// submit queues write and returns where its error will arrive. It never
// blocks, so Update never waits on a write.
func (w *writer) submit(write func() error) <-chan error {
	result := make(chan error, 1)
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.started {
		w.waiting = sync.NewCond(&w.mu)
		w.started = true
		go w.run()
	}
	w.queue = append(w.queue, func() { result <- write() })
	w.waiting.Signal()
	return result
}

// run is the one worker: it takes the oldest write and runs it outside the
// lock, forever. It lives as long as the model.
func (w *writer) run() {
	for {
		w.mu.Lock()
		for len(w.queue) == 0 {
			w.waiting.Wait()
		}
		next := w.queue[0]
		w.queue = w.queue[1:]
		w.mu.Unlock()
		next()
	}
}
