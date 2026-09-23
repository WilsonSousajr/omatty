package ui_test

import (
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
}

func (r *turnRecorder) funcs() ui.TurnFuncs {
	return ui.TurnFuncs{
		Snap: func(s registry.Session) error {
			r.Snapped = append(r.Snapped, s.ID)
			return r.SnapErr
		},
		Diff: func(registry.Session, string) (review.Diff, error) {
			r.DiffCalls++
			return r.Diff, r.DiffErr
		},
		Drop: func(s registry.Session, root string) error {
			r.Dropped = append(r.Dropped, [2]string{s.ID, root})
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
