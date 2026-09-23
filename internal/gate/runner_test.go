package gate_test

import (
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

// waitReport reads one report, failing rather than hanging the suite.
func waitReport(t *testing.T, r *gate.Runner) gate.Report {
	t.Helper()
	select {
	case rep := <-r.Reports():
		return rep
	case <-time.After(30 * time.Second):
		t.Fatal("no report arrived")
		return gate.Report{}
	}
}

func TestRunner_reportsTheRunItWasGiven(t *testing.T) {
	r := gate.NewRunner(2)
	defer r.Close()

	r.Start("s1", t.TempDir(), []gate.Step{{Name: "ok", Run: "true"}})

	rep := waitReport(t, r)
	if rep.ID != "s1" {
		t.Errorf("Report.ID = %q, want s1", rep.ID)
	}
	if len(rep.Results) != 1 || rep.Results[0].Verdict != gate.Pass {
		t.Errorf("Report.Results = %+v, want one Pass", rep.Results)
	}
}

// A working directory that is gone is the caller's error, and it has to reach
// the caller rather than vanish into the goroutine.
func TestRunner_surfacesAnUnusableDirectory(t *testing.T) {
	r := gate.NewRunner(1)
	defer r.Close()

	r.Start("s1", "/no/such/directory/anywhere", []gate.Step{{Name: "ok", Run: "true"}})

	if rep := waitReport(t, r); rep.Err == nil {
		t.Error("Report.Err = nil, want the directory failure surfaced")
	}
}

// Re-gating a session while its gate is still running must leave exactly one
// answer - the new one. A superseded run reporting late would overwrite the
// card with a verdict about code that has since changed.
func TestRunner_restartingASession_reportsOnlyTheNewRun(t *testing.T) {
	r := gate.NewRunner(2)
	defer r.Close()
	dir := t.TempDir()

	r.Start("s1", dir, []gate.Step{{Name: "slow", Run: "sleep 30"}})
	r.Start("s1", dir, []gate.Step{{Name: "quick", Run: "true"}})

	rep := waitReport(t, r)
	if rep.Results[0].Step.Name != "quick" {
		t.Errorf("first report is from %q, want the superseding run", rep.Results[0].Step.Name)
	}
	select {
	case extra := <-r.Reports():
		t.Errorf("a superseded run reported anyway: %+v", extra)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRunner_cancelStopsAnInFlightRun(t *testing.T) {
	r := gate.NewRunner(1)
	defer r.Close()

	r.Start("s1", t.TempDir(), []gate.Step{{Name: "slow", Run: "sleep 30"}})
	r.Cancel("s1")

	select {
	case rep := <-r.Reports():
		t.Errorf("a cancelled run reported anyway: %+v", rep)
	case <-time.After(500 * time.Millisecond):
	}
}

// Close must not leave a goroutine writing into a channel nobody reads, and
// must not panic on a second call - quitting can race a shutdown already
// under way.
func TestRunner_closeIsSafeTwiceAndStopsWork(t *testing.T) {
	r := gate.NewRunner(2)
	r.Start("s1", t.TempDir(), []gate.Step{{Name: "slow", Run: "sleep 30"}})

	r.Close()
	r.Close()

	if _, open := <-r.Reports(); open {
		t.Error("Reports() delivered after Close()")
	}
}

// Starting after Close is a no-op rather than a panic: the UI can queue a gate
// on the same tick a quit is processed.
func TestRunner_startAfterCloseIsIgnored(t *testing.T) {
	r := gate.NewRunner(1)
	r.Close()

	r.Start("s1", t.TempDir(), []gate.Step{{Name: "ok", Run: "true"}})
}
