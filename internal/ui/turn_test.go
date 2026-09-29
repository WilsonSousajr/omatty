package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// turnRecorder is a named fake for ui.TurnFuncs (#311).
type turnRecorder struct {
	Snapped   []string
	SnapErr   error
	Diff      review.Diff
	DiffErr   error
	DiffCalls int
	Dropped   [][2]string // id, projectRoot
	Events    []string    // "snap <id>" and "drop <id>", in the order they ran
}

func (r *turnRecorder) funcs() ui.TurnFuncs {
	return ui.TurnFuncs{
		Snap: func(s registry.Session) error {
			r.Snapped = append(r.Snapped, s.ID)
			r.Events = append(r.Events, "snap "+s.ID)
			return r.SnapErr
		},
		Diff: func(registry.Session, string) (review.Diff, error) {
			r.DiffCalls++
			return r.Diff, r.DiffErr
		},
		Drop: func(s registry.Session, root string) error {
			r.Dropped = append(r.Dropped, [2]string{s.ID, root})
			r.Events = append(r.Events, "drop "+s.ID)
			return nil
		},
	}
}

func hookPrompt(id string) ui.StatusMsg {
	return ui.StatusMsg{SessionID: id, Kind: watcher.PromptSubmitted, At: time.Now(), Hook: true}
}

func modelWithTurn(t *testing.T, tr *turnRecorder) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Turn = tr.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func TestModel_aHookPromptSnapsTheTurnBaseline_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m := modelWithTurn(t, tr)

	_, cmd := m.Update(hookPrompt("s1"))
	settle(m, cmd)

	if len(tr.Snapped) != 1 || tr.Snapped[0] != "s1" {
		t.Errorf("snapped = %v, want [s1]", tr.Snapped)
	}
}

// The tailer reports PromptSubmitted for every tool result; a snapshot on
// one would move the baseline mid-turn and drop the turn's earlier edits.
func TestModel_aTailerPromptDoesNotSnap_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m := modelWithTurn(t, tr)
	msg := hookPrompt("s1")
	msg.Hook = false

	_, cmd := m.Update(msg)
	settle(m, cmd)

	if len(tr.Snapped) != 0 {
		t.Errorf("snapped %v on a tailer event", tr.Snapped)
	}
}

// Two prompts inside one snapshot: the second is dropped, so the diff shows
// more than one turn rather than less.
func TestModel_aPromptWhileASnapIsInFlightStartsNothing_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m := modelWithTurn(t, tr)

	_, first := m.Update(hookPrompt("s1"))
	_, second := m.Update(hookPrompt("s1"))
	settle(m, first)
	settle(m, second)

	if len(tr.Snapped) != 1 {
		t.Errorf("snapped %d times, want 1", len(tr.Snapped))
	}
}

func TestModel_archiveDropsTheTurnBaseline_issue311(t *testing.T) {
	tr := &turnRecorder{}
	r := &recordArchive{}
	terms, _ := fakeTerms(t)
	st := worktreeState()
	r.State = st
	d := baseDeps(st, terms)
	d.Archive, d.TailStop, d.RemoveWorktree = r.archive, r.stopTail, r.removeWorktree
	d.Turn = tr.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	openArchive(t, m, "s1")

	pressAndSettle(m, key('y'))

	if len(tr.Dropped) != 1 || tr.Dropped[0] != [2]string{"s1", "/p/omatty"} {
		t.Errorf("dropped = %v, want [[s1 /p/omatty]]", tr.Dropped)
	}
}

// A snapshot in flight when its session is archived used to recreate the ref
// archive had just deleted, for a session that no longer exists, and nothing
// ever removed it (#350). Only a directory that survives the archive can do
// this - a main checkout, as s1 is here - and there is deliberately no boot
// sweep, since two omatty HOMEs can share one repository. So a success that
// lands for a forgotten session drops its baseline again.
func TestModel_aSnapshotLandingAfterArchiveIsDroppedAgain_issue350(t *testing.T) {
	tr := &turnRecorder{}
	r := &recordArchive{}
	terms, _ := fakeTerms(t)
	st := worktreeState()
	r.State = st
	d := baseDeps(st, terms)
	d.Archive, d.TailStop, d.RemoveWorktree = r.archive, r.stopTail, r.removeWorktree
	d.Turn = tr.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	_, inFlight := m.Update(hookPrompt("s1")) // the snapshot, not yet run
	openArchive(t, m, "s1")
	pressAndSettle(m, key('y'))

	settle(m, inFlight) // it lands after the archive's drop

	want := []string{"drop s1", "snap s1", "drop s1"}
	if strings.Join(tr.Events, ", ") != strings.Join(want, ", ") {
		t.Errorf("events = %v, want %v: the late snapshot's ref must be dropped after it", tr.Events, want)
	}
	if last := tr.Dropped[len(tr.Dropped)-1]; last != [2]string{"s1", "/p/omatty"} {
		t.Errorf("the second drop was %v, want [s1 /p/omatty]", last)
	}
}

// A load answered out of order does not paint over a newer one (#352). A turn
// load started before a new baseline, but answered after the reload that
// baseline triggers, painted a diff spanning two turns until the next reload;
// the whole-session diff had the same race. Each load is numbered, and only
// the latest answer is drawn.
func TestModel_anOlderLoadAnsweredLastIsDropped_issue352(t *testing.T) {
	tr := &turnRecorder{}
	m, _ := modelWithTurnDiff(t, tr)
	pressAndSettle(m, key('t'))
	_, older := m.Update(key('r'))
	_, newer := m.Update(key('r'))

	tr.Diff = turnDiffParsed(t) // the newer load sees this turn alone
	newerAnswers := drainCmd(newer)
	tr.Diff = sampleDiffParsed(t) // the older one saw two turns' changes
	olderAnswers := drainCmd(older)
	for _, msg := range append(newerAnswers, olderAnswers...) {
		m.Update(msg)
	}

	body := stripSGR(m.View().Content) // text: a Go line is syntax-coloured since #435
	if strings.Contains(body, "new.txt") || !strings.Contains(body, "c := 4") {
		t.Errorf("the older answer, delivered last, painted over the newer:\n%s", body)
	}
}
