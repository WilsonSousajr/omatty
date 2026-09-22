package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// sweepT0 is when the model under test booted.
var sweepT0 = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

// sweepRig is a model with a movable clock and a record of every session
// whose process was ended.
type sweepRig struct {
	m     *Model
	now   *time.Time
	ended *[]string
}

// newSweepRig boots three running sessions at sweepT0 with s1 selected.
func newSweepRig(t *testing.T, idle time.Duration) sweepRig {
	t.Helper()
	now, ended := sweepT0, []string{}
	st := registry.State{
		Projects: []registry.Project{{Name: "p", Root: "/p"}},
		Sessions: []registry.Session{{ID: "s1", Project: "p", Title: "one"},
			{ID: "s2", Project: "p", Title: "two"}, {ID: "s3", Project: "p", Title: "three"}},
	}
	terms := map[string]termwrap.Terminal{}
	for _, sess := range st.Sessions {
		terms[sess.ID] = termwrap.NewFake(sess.Title)
	}
	m := NewModel(Deps{State: st, Terms: terms, IdleStop: idle,
		Clock: func() time.Time { return now },
		Stop:  func(id string) error { ended = append(ended, id); return nil },
		Start: func(registry.Session, int, int) (termwrap.Terminal, error) { return termwrap.NewFake(""), nil },
	})
	return sweepRig{m: m, now: &now, ended: &ended}
}

// settled records a transcript turn for id at the given time, as the tailer
// would on its first read.
func (r sweepRig) settled(id string, status watcher.Status, at time.Time) {
	r.m.status[id] = watcher.SessionState{Status: status, At: at}
}

// sweep runs one sweep and the stops it scheduled, returning who was stopped.
func (r sweepRig) sweep() []string {
	before := len(*r.ended)
	for _, cmd := range r.m.sweep() {
		cmd()
	}
	return (*r.ended)[before:]
}

func (r sweepRig) running(id string) bool { return r.m.terms[id] != nil }

// A session whose last turn is older than the threshold, and which omatty
// neither started nor saw typed into since, is stopped as ctrl+o s would:
// process ended, row kept (#319).
func TestSweep_StopsAQuietSession_issue319(t *testing.T) {
	r := newSweepRig(t, time.Hour)
	r.settled("s2", watcher.StatusDone, sweepT0.Add(-3*time.Hour))
	*r.now = sweepT0.Add(2 * time.Hour)

	stopped := r.sweep()

	if r.running("s2") || !slicesContain(stopped, "s2") {
		t.Errorf("stopped %v, want s2 among them", stopped)
	}
	if _, ok := r.m.sessionIndex("s2"); !ok {
		t.Error("the sweep forgot s2's row; it must only stop the process")
	}
}

// The pane the operator is looking at never goes blank under their hands.
func TestSweep_SparesTheSelectedSession_issue319(t *testing.T) {
	r := newSweepRig(t, time.Hour)
	r.settled("s1", watcher.StatusDone, sweepT0.Add(-3*time.Hour))
	*r.now = sweepT0.Add(2 * time.Hour)

	if stopped := r.sweep(); slicesContain(stopped, "s1") || !r.running("s1") {
		t.Errorf("the sweep stopped the selected s1: %v", stopped)
	}
}

// A turn in flight, or a session waiting on the operator's answer, is never
// quiet, however old its last settled turn (#319).
func TestSweep_SparesABusyOrWaitingSession_issue319(t *testing.T) {
	for _, status := range []watcher.Status{watcher.StatusThinking, watcher.StatusTool, watcher.StatusWaiting} {
		r := newSweepRig(t, time.Hour)
		r.settled("s2", status, sweepT0.Add(-3*time.Hour))
		*r.now = sweepT0.Add(2 * time.Hour)

		if stopped := r.sweep(); slicesContain(stopped, "s2") {
			t.Errorf("the sweep stopped s2 while it was %q", status)
		}
	}
}

// A session started moments ago has no transcript at all; its start is its
// floor (#319).
func TestSweep_SparesAJustStartedSession_issue319(t *testing.T) {
	r := newSweepRig(t, time.Hour)
	*r.now = sweepT0.Add(30 * time.Minute)

	if stopped := r.sweep(); slicesContain(stopped, "s3") {
		t.Errorf("the sweep stopped s3, started 30m ago with no transcript: %v", stopped)
	}
}

// A session resumed with enter is fresh again, whatever its transcript says.
func TestSweep_SparesASessionJustResumed_issue319(t *testing.T) {
	r := newSweepRig(t, time.Hour)
	r.settled("s2", watcher.StatusDone, sweepT0.Add(-3*time.Hour))
	*r.now = sweepT0.Add(2 * time.Hour)
	r.sweep()
	r.m.sidebar.SelectByID("s2")
	r.m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	r.m.sidebar.SelectByID("s1")
	*r.now = r.now.Add(10 * time.Minute)

	if stopped := r.sweep(); slicesContain(stopped, "s2") {
		t.Errorf("the sweep stopped s2 ten minutes after it was resumed: %v", stopped)
	}
}

// The transcript is not the only evidence of use. Typing into a pane is use,
// whatever the transcript says - a /clear'd session before #316 kept a frozen
// transcript while its work went on, and this is the floor that covered it.
func TestSweep_SparesASessionBeingTypedInto_issue316(t *testing.T) {
	r := newSweepRig(t, time.Hour)
	r.settled("s2", watcher.StatusDone, sweepT0.Add(-72*time.Hour))
	*r.now = sweepT0.Add(2 * time.Hour)
	r.m.sidebar.SelectByID("s2")
	r.m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	r.m.sidebar.SelectByID("s1")
	*r.now = r.now.Add(10 * time.Minute)

	if stopped := r.sweep(); slicesContain(stopped, "s2") {
		t.Errorf("the sweep stopped s2, typed into ten minutes ago: %v", stopped)
	}
}

// A session with no process has nothing to stop; a second sweep leaves it.
func TestSweep_LeavesAStoppedSessionAlone_issue319(t *testing.T) {
	r := newSweepRig(t, time.Hour)
	r.settled("s2", watcher.StatusDone, sweepT0.Add(-3*time.Hour))
	*r.now = sweepT0.Add(2 * time.Hour)
	r.sweep()

	if again := r.sweep(); slicesContain(again, "s2") {
		t.Errorf("the second sweep stopped s2 again: %v", again)
	}
}

// Off costs nothing, not even a timer; on, the tick is the threshold, capped
// at a minute so a long threshold is still honoured to the minute (#319).
func TestScheduleSweep_OnlyWhenOn_issue319(t *testing.T) {
	if cmd := newSweepRig(t, 0).m.scheduleSweep(); cmd != nil {
		t.Error("a zero threshold scheduled a sweep tick")
	}
	if cmd := newSweepRig(t, time.Hour).m.scheduleSweep(); cmd == nil {
		t.Error("a one-hour threshold scheduled no sweep tick")
	}
	if got := sweepEvery(3 * time.Second); got != 3*time.Second {
		t.Errorf("sweepEvery(3s) = %v, want 3s", got)
	}
	if got := sweepEvery(3 * time.Hour); got != time.Minute {
		t.Errorf("sweepEvery(3h) = %v, want 1m", got)
	}
}

// Each tick arms the next, including one that stopped nothing.
func TestOnSweepTick_ReArms_issue319(t *testing.T) {
	if cmd := newSweepRig(t, time.Hour).m.onSweepTick(); cmd == nil {
		t.Error("onSweepTick returned nil; the sweep would run once and never again")
	}
}
