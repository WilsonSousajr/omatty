package gate

import (
	"context"
	"fmt"
	"sync"
)

// reportBuffer is generous because a report is expensive to produce and cheap
// to hold: dropping one would leave a card showing a verdict about code that
// has since changed.
const reportBuffer = 64

// Report is the outcome of one session's gate run.
//
// Err is set only when the run could not happen at all - an unusable working
// directory, or a panic. A gate that ran and failed is not an error; it is
// Results, and the caller renders it.
type Report struct {
	ID      string
	Results []StepResult
	Err     error
}

// Runner runs gates for many sessions at once, bounded.
//
//	r := gate.NewRunner(cfg.Gate.MaxParallel)
//	r.Start(sess.ID, sess.Dir, proj.Gate)
//	for rep := range r.Reports() { ... }
//
// It mirrors watcher.Watch deliberately - Start, a Reports channel, Close - so
// the UI consumes gate results the same way it already consumes status events.
//
// The bound is the point. A gate is the most expensive thing omatty runs, and
// four concurrent `go test ./... -race` will make the machine unusable; a
// laggy TUI is the one thing that would make this feature worse than running
// the gate by hand.
//
// Runs happen in the session's own directory, which for a worktree session is
// that worktree, so parallel sessions gate independently without clobbering
// each other. That is the payoff of the worktree model.
type Runner struct {
	slots   chan struct{}
	reports chan Report
	done    chan struct{}

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	closed  bool
	// inflight is what makes closing reports safe without swallowing a panic:
	// Close waits for every sender to finish before it closes the channel.
	inflight sync.WaitGroup

	// run is the work itself, injected so a test can supply one that panics.
	// Invariant 6 cannot be exercised against the real Run, which does not.
	run func(context.Context, string, []Step) ([]StepResult, error)
}

// NewRunner returns a Runner allowing at most limit gates at once. A limit
// below 1 is raised to 1: zero would mean a gate that never runs, which is
// never what a configuration file meant.
func NewRunner(limit int) *Runner { return newRunnerWith(limit, Run) }

func newRunnerWith(limit int, run func(context.Context, string, []Step) ([]StepResult, error)) *Runner {
	if limit < 1 {
		limit = 1
	}
	return &Runner{
		slots:   make(chan struct{}, limit),
		reports: make(chan Report, reportBuffer),
		done:    make(chan struct{}),
		cancels: make(map[string]context.CancelFunc),
		run:     run,
	}
}

// Reports delivers one report per completed run. It is closed by Close.
func (r *Runner) Reports() <-chan Report { return r.reports }

// Start gates session id in dir, superseding any run already in flight for it.
// Re-gating while a run is going must leave exactly one answer, and it has to
// be the new one.
func (r *Runner) Start(id, dir string, steps []Step) {
	ctx, ok := r.begin(id)
	if !ok {
		return
	}
	r.inflight.Add(1)
	go r.gate(ctx, id, dir, steps)
}

// Cancel stops session id's run without starting another.
func (r *Runner) Cancel(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked(id)
}

// Close cancels everything in flight and closes Reports. Safe to call twice:
// quitting can race a shutdown already under way.
//
// The order matters. done is closed first so a sender parked on a full buffer
// wakes and gives up; only then does Close wait for every run to finish, and
// only then is reports closed. A sender can therefore never write to a closed
// channel, which is why nothing here recovers from a panic it caused itself.
func (r *Runner) Close() {
	if !r.beginClose() {
		return
	}
	close(r.done)
	r.inflight.Wait()
	close(r.reports)
}

// beginClose marks the Runner closed and cancels every run, reporting false if
// someone else already did.
func (r *Runner) beginClose() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	r.closed = true
	for id := range r.cancels {
		r.stopLocked(id)
	}
	return true
}

// begin registers a run, cancelling whatever that session had. It reports
// false once closed, so a gate queued on the same tick as a quit is ignored
// rather than panicking on a closed channel.
func (r *Runner) begin(id string) (context.Context, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, false
	}
	r.stopLocked(id)
	ctx, cancel := context.WithCancel(context.Background())
	r.cancels[id] = cancel
	return ctx, true
}

// stopLocked cancels id's run if it has one. Caller holds mu.
func (r *Runner) stopLocked(id string) {
	if cancel, running := r.cancels[id]; running {
		cancel()
		delete(r.cancels, id)
	}
}

// gate is one run: wait for a slot, do the work, report it.
func (r *Runner) gate(ctx context.Context, id, dir string, steps []Step) {
	defer r.inflight.Done()
	defer r.recoverRun(id)
	select {
	case r.slots <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-r.slots }()

	results, err := r.run(ctx, dir, steps)
	if ctx.Err() != nil {
		return // superseded or cancelled: its answer is no longer wanted
	}
	r.send(Report{ID: id, Results: results, Err: err})
}

// recoverRun keeps one session's panic to that session (invariant 6).
func (r *Runner) recoverRun(id string) {
	panicked := recover()
	if panicked == nil {
		return
	}
	r.send(Report{ID: id, Err: fmt.Errorf("gate: run for session %s panicked: %v", id, panicked)})
}

// send delivers a report, or gives up if the Runner is closing - never blocks
// on a channel nobody will read again.
func (r *Runner) send(rep Report) {
	select {
	case r.reports <- rep:
	case <-r.done:
	}
}
