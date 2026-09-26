package ui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// turnReverter is a named RevertFunc fake: it counts files and records the
// sessions it was asked to revert.
type turnReverter struct {
	Files    int
	CountErr error
	Err      error
	Reverted []string
}

func (r *turnReverter) revert(sess registry.Session) (int, error) {
	r.Reverted = append(r.Reverted, sess.ID)
	return r.Files, r.Err
}

func (r *turnReverter) count(_ registry.Session) (int, error) { return r.Files, r.CountErr }

func modelWithRevert(t *testing.T, rev *turnReverter) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Turn.Revert, d.Turn.Count = rev.revert, rev.count
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	// A turn has to have started for there to be a baseline to go back to.
	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())
	return m
}

// Akita's safety net leg #334 is built on: undoing a bad change must be cheaper
// than preventing it. One key, one confirmation, on the selected session.
func TestModel_uAsksBeforeRevertingAndThenReverts_issue334(t *testing.T) {
	rev := &turnReverter{Files: 3}
	m := modelWithRevert(t, rev)

	leader(m, key('u'))

	view := m.View().Content
	if !strings.Contains(view, "revert") {
		t.Fatalf("u did not open a confirmation:\n%s", view)
	}
	if !strings.Contains(view, "3") {
		t.Errorf("the confirmation does not name how many files would go:\n%s", view)
	}
	if len(rev.Reverted) != 0 {
		t.Fatal("u reverted before the operator answered")
	}

	pressDeliver(m, key('u'))

	if len(rev.Reverted) != 1 || rev.Reverted[0] != "s1" {
		t.Errorf("reverted %v, want the selected session once", rev.Reverted)
	}
}

// esc is the way out, and it must leave the worktree alone.
func TestModel_escLeavesTheRevertUndone_issue334(t *testing.T) {
	rev := &turnReverter{Files: 3}
	m := modelWithRevert(t, rev)

	leader(m, key('u'))
	press(m, special(tea.KeyEscape))

	if len(rev.Reverted) != 0 {
		t.Errorf("esc reverted anyway: %v", rev.Reverted)
	}
	if strings.Contains(m.View().Content, "will be discarded") {
		t.Error("the confirmation is still open after esc")
	}
}

// The destructive answer must not be the key the hand reaches for - the
// argument archiveChoices already makes for `w`. Enter must not do it.
func TestModel_enterDoesNotConfirmARevert_issue334(t *testing.T) {
	rev := &turnReverter{Files: 3}
	m := modelWithRevert(t, rev)

	leader(m, key('u'))
	press(m, special(tea.KeyEnter))

	if len(rev.Reverted) != 0 {
		t.Errorf("enter confirmed a destructive action: %v", rev.Reverted)
	}
}

// Refused mid-turn: a revert while the agent is writing would race it, and the
// baseline it would restore to is the turn that is still running.
func TestModel_uRefusesWhileATurnIsInFlight_issue334(t *testing.T) {
	rev := &turnReverter{Files: 3}
	m := modelWithRevert(t, rev)
	statusDeliver(m, "s1", watcher.ToolStarted, time.Now())

	leader(m, key('u'))

	if strings.Contains(m.View().Content, "will be discarded") {
		t.Error("a revert was offered mid-turn")
	}
	if got := m.View().Content; !strings.Contains(got, "mid-turn") {
		t.Errorf("nothing said why it was refused:\n%s", got)
	}
}

// No baseline, nothing to go back to. #311's own notice vocabulary.
func TestModel_uSaysWhenThereIsNoBaseline_issue334(t *testing.T) {
	rev := &turnReverter{CountErr: review.ErrNoTurn}
	m := modelWithRevert(t, rev)

	leader(m, key('u'))

	if len(rev.Reverted) != 0 {
		t.Fatal("reverted with no baseline")
	}
	if got := m.View().Content; !strings.Contains(got, "no turn") {
		t.Errorf("nothing said there was no baseline:\n%s", got)
	}
}

// A turn that changed nothing has nothing to discard, so the question is not
// worth asking.
func TestModel_uSaysWhenTheTurnChangedNothing_issue334(t *testing.T) {
	rev := &turnReverter{Files: 0}
	m := modelWithRevert(t, rev)

	leader(m, key('u'))

	if len(rev.Reverted) != 0 {
		t.Fatal("reverted a turn that changed nothing")
	}
	if got := m.View().Content; !strings.Contains(got, "nothing") {
		t.Errorf("nothing said the turn was empty:\n%s", got)
	}
}

// A failed revert reaches the operator: a worktree half restored in silence is
// the one failure they cannot see from inside the terminal.
func TestModel_aFailedRevertIsReported_issue334(t *testing.T) {
	rev := &turnReverter{Files: 2, Err: errors.New("disk full")}
	m := modelWithRevert(t, rev)

	leader(m, key('u'))
	pressDeliver(m, key('u'))

	if got := m.View().Content; !strings.Contains(got, "disk full") {
		t.Errorf("the failure was swallowed:\n%s", got)
	}
}

// It says what it did, with the number the confirmation showed.
func TestModel_aRevertSaysWhatItDid_issue334(t *testing.T) {
	rev := &turnReverter{Files: 2}
	m := modelWithRevert(t, rev)

	leader(m, key('u'))
	pressDeliver(m, key('u'))

	if got := m.View().Content; !strings.Contains(got, "2") || !strings.Contains(got, "revert") {
		t.Errorf("the footer does not report the revert:\n%s", got)
	}
}

// Regression, found by #334's own real-PTY smoke test: watcher.Status is a
// string, so a session nothing has reported on holds "" rather than
// StatusIdle - and atRest("") is false. u refused every such session with
// "cannot revert mid-turn", which is every session at boot and every stopped
// one: exactly the ones a revert is most useful for. No unit test caught it
// because every fixture reports a status first.
func TestModel_uRevertsASessionThatHasReportedNoStatus_issue334(t *testing.T) {
	rev := &turnReverter{Files: 2}
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Turn.Revert, d.Turn.Count = rev.revert, rev.count
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	// Deliberately no statusDeliver: this is a model at boot.

	leader(m, key('u'))

	if got := m.View().Content; strings.Contains(got, "mid-turn") {
		t.Fatalf("a session with no reported status was refused as mid-turn:\n%s", got)
	}
	pressDeliver(m, key('u'))
	if len(rev.Reverted) != 1 {
		t.Errorf("reverted %v, want the session reverted once", rev.Reverted)
	}
}
