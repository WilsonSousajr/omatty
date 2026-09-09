package ui_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func modelWithStat(t *testing.T) (*ui.Model, *FakeStat) {
	t.Helper()
	terms, _ := fakeTerms(t)
	stat := &FakeStat{Stats: map[string]review.Stat{"s1": {Branch: "main", Added: 12, Removed: 3}}}
	d := baseDeps(twoProjectState(), terms)
	d.Stat = stat.Stat
	return ui.NewModel(d), stat
}

func TestModel_AStatTickPollsEverySessionWithItsProjectRoot_issue180(t *testing.T) {
	m, stat := modelWithStat(t)

	deliver(m, m.PollAll())

	if len(stat.Asked) != 3 || stat.Roots[0] == "" {
		t.Fatalf("asked %v with roots %v, want all three sessions and their project roots", stat.Asked, stat.Roots)
	}
	if st, ok := m.RepoStatOf("s1"); !ok || st.Branch != "main" || st.Added != 12 {
		t.Errorf("RepoStatOf(s1) = %+v, %v; want the polled stat", st, ok)
	}
}

func TestModel_AStatTickReArmsItself_issue180(t *testing.T) {
	m, _ := modelWithStat(t)
	if _, cmd := m.Update(ui.StatTickMsg(fixedNow)); cmd == nil {
		t.Error("a stat tick returned no command; the poll would run once and never again")
	}
}

// A session with a poll in flight is not polled again until it answers.
func TestModel_APollInFlightIsNotRepeated_issue180(t *testing.T) {
	m, stat := modelWithStat(t)
	first := m.PollAll()

	deliver(m, m.PollAll())
	if len(stat.Asked) != 0 {
		t.Fatalf("the second poll asked %v while the first was in flight", stat.Asked)
	}
	deliver(m, first)
	if len(stat.Asked) != 3 {
		t.Errorf("after the first poll landed, asked %v, want all three", stat.Asked)
	}
	deliver(m, m.PollAll())
	if len(stat.Asked) != 6 {
		t.Errorf("after the answers landed a new poll asked %d in total, want 6", len(stat.Asked))
	}
}

// Done and waiting are the moments the numbers change; a tool start is not.
func TestModel_DoneAndWaitingPollAtOnce_issue180(t *testing.T) {
	m, stat := modelWithStat(t)
	statusDeliver(m, "s1", watcher.ToolStarted, fixedNow)
	if len(stat.Asked) != 0 {
		t.Fatalf("a tool start polled %v", stat.Asked)
	}
	statusDeliver(m, "s1", watcher.TurnEnded, fixedNow.Add(time.Second))
	statusDeliver(m, "s2", watcher.PermissionRequested, fixedNow.Add(2*time.Second))
	if strings.Join(stat.Asked, " ") != "s1 s2" {
		t.Errorf("asked %v, want s1 on done then s2 on waiting", stat.Asked)
	}
}

// A failure keeps the last stat, is logged once per session, and logs again
// only after a success in between.
func TestModel_AFailedPollKeepsTheLastStatAndWarnsOnce_issue180(t *testing.T) {
	var log bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	m, stat := modelWithStat(t)
	deliver(m, m.PollAll())
	stat.Err = errors.New("boom")

	deliver(m, m.PollAll())
	deliver(m, m.PollAll())

	if st, ok := m.RepoStatOf("s1"); !ok || st.Added != 12 {
		t.Errorf("RepoStatOf(s1) = %+v, %v; want the last good stat kept", st, ok)
	}
	if n := strings.Count(log.String(), "reading repo stat"); n != 3 {
		t.Errorf("logged %d warnings after two failed rounds over three sessions, want 3 (once each)", n)
	}
	stat.Err = nil
	deliver(m, m.PollAll())
	stat.Err = errors.New("boom again")
	deliver(m, m.PollAll())
	if n := strings.Count(log.String(), "reading repo stat"); n != 6 {
		t.Errorf("logged %d warnings, want 6: the once-flag clears on success", n)
	}
}

func TestModel_NoStatReaderPollsNothing_issue180(t *testing.T) {
	m, _ := modelWithFakes(t)
	if cmd := m.PollAll(); cmd != nil {
		deliver(m, cmd) // must not panic
	}
	if _, ok := m.RepoStatOf("s1"); ok {
		t.Error("a model with no stat reader holds a stat")
	}
}
