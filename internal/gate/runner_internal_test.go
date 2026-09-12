package gate

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

// These two properties cannot be exercised against the real Run - it does not
// panic, and timing it would be a sleep pretending to be a test - so the work
// function is injected. Everything else about the Runner is tested against the
// real thing in runner_test.go.

// The bound is the whole reason the Runner exists. Four concurrent
// `go test ./... -race` make the machine unusable, and a laggy TUI would make
// this feature worse than running the gate by hand.
func TestRunner_neverExceedsItsParallelismBound(t *testing.T) {
	const limit, jobs = 2, 4
	entered := make(chan struct{}, jobs)
	release := make(chan struct{})
	var current, peak int32

	work := func(context.Context, string, []Step) ([]StepResult, error) {
		raise(&peak, atomic.AddInt32(&current, 1))
		entered <- struct{}{}
		<-release
		atomic.AddInt32(&current, -1)
		return nil, nil
	}

	r := newRunnerWith(limit, work)
	defer r.Close()
	for i := range jobs {
		r.Start(fmt.Sprintf("s%d", i), t.TempDir(), nil)
	}

	// Each run records its concurrency before signalling, so once limit
	// signals have arrived, limit runs are provably in flight at once.
	for range limit {
		<-entered
	}
	if got := atomic.LoadInt32(&peak); got != limit {
		t.Errorf("peak concurrency = %d, want %d: the bound must permit parallelism, not serialise", got, limit)
	}

	close(release)
	for range jobs {
		<-r.Reports()
	}
	if got := atomic.LoadInt32(&peak); got > limit {
		t.Errorf("peak concurrency reached %d, want at most %d", got, limit)
	}
}

// raise lifts peak to n if n is larger.
func raise(peak *int32, n int32) {
	for {
		seen := atomic.LoadInt32(peak)
		if n <= seen || atomic.CompareAndSwapInt32(peak, seen, n) {
			return
		}
	}
}

// Invariant 6: one session's panic must not take the app, or any other
// session, with it.
func TestRunner_aPanickingRun_marksOnlyItsOwnSession(t *testing.T) {
	work := func(_ context.Context, dir string, _ []Step) ([]StepResult, error) {
		if strings.HasSuffix(dir, "boom") {
			panic("deliberate, from a test")
		}
		return []StepResult{{Step: Step{Name: "ok"}, Verdict: Pass}}, nil
	}

	r := newRunnerWith(2, work)
	defer r.Close()
	r.Start("panics", "/tmp/boom", nil)
	r.Start("fine", "/tmp/fine", nil)

	got := map[string]Report{}
	for range 2 {
		rep := <-r.Reports()
		got[rep.ID] = rep
	}

	if err := got["panics"].Err; err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Errorf("panicking session Err = %v, want one saying so", err)
	}
	if !strings.Contains(fmt.Sprint(got["panics"].Err), "panics") {
		t.Errorf("Err = %v, want it to name the session", got["panics"].Err)
	}
	if rep := got["fine"]; rep.Err != nil || len(rep.Results) != 1 {
		t.Errorf("the other session was disturbed: %+v", rep)
	}
}

// A limit of zero would mean a gate that never runs, which is never what a
// configuration file meant.
func TestNewRunner_raisesAnImpossibleLimit(t *testing.T) {
	for _, limit := range []int{0, -3} {
		if got := cap(newRunnerWith(limit, Run).slots); got != 1 {
			t.Errorf("newRunnerWith(%d) slots = %d, want 1", limit, got)
		}
	}
}
