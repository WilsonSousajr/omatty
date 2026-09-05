package ui_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/ui"
)

// recordEnd is a named fake for the holder's Stop, which ends the claude behind
// a session. Separate from recordArchive's tailer stop: one ends a goroutine
// omatty owns, the other ends a process that outlives omatty, and a test that
// confused them would prove nothing (#43).
type recordEnd struct{ Ended []string }

func (r *recordEnd) stop(sessionID string) error {
	r.Ended = append(r.Ended, sessionID)
	return nil
}

func modelWithEnd(t *testing.T, r *recordEnd) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	st := worktreeState()
	arch := &recordArchive{State: st}
	d := baseDeps(st, terms)
	d.Archive, d.TailStop, d.RemoveWorktree = arch.archive, arch.stopTail, arch.removeWorktree
	d.Stop = r.stop
	return ui.NewModel(d)
}

// Archiving is the one place omatty deliberately ends a claude. Under dtach the
// process outlives the PTY, so closing the terminal is no longer enough: the
// claude would keep running behind a socket with no row in state.json, reachable
// from neither the sidebar nor the registry (#40, #43).
func TestModel_archiveEndsTheDetachedProcess_issue43(t *testing.T) {
	r := &recordEnd{}
	m := modelWithEnd(t, r)
	openArchive(t, m, "s2")

	press(m, key('y'))

	if len(r.Ended) != 1 || r.Ended[0] != "s2" {
		t.Errorf("ended = %v, want the archived session s2", r.Ended)
	}
}

// The other half of the same rule, and the one the milestone exists for:
// quitting must leave every claude running. A Stop on the quit path would undo
// persistence entirely while every test above still passed (#43).
func TestModel_quittingEndsNoProcess_issue43(t *testing.T) {
	r := &recordEnd{}
	m := modelWithEnd(t, r)

	press(m, ctrl('o'))
	press(m, key('q'))

	if len(r.Ended) != 0 {
		t.Errorf("ended = %v on quit, want nothing: quitting detaches, it does not kill", r.Ended)
	}
}

// Without dtach omatty still works, so this is a notice rather than an error,
// and it has to name the fix: an operator who does not know what dtach is
// cannot act on "sessions will not survive quit" alone (#43).
func TestModel_noticesThatSessionsWillNotSurviveQuit_issue43(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Notice = "dtach not found: sessions will not survive quit (brew install dtach)"

	got := ui.NewModel(d).View().Content

	if !strings.Contains(got, "brew install dtach") {
		t.Errorf("the frame does not carry the notice:\n%s", got)
	}
}

// It is a once-per-session notice, not a permanent banner: the footer's keymap
// is worth more than a warning the operator has already read, so the first
// keypress acknowledges it exactly as it acknowledges an error (#43).
func TestModel_theNoticeGivesWayToTheKeymapOnTheFirstKey_issue43(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Notice = "dtach not found: sessions will not survive quit (brew install dtach)"
	m := ui.NewModel(d)

	press(m, key('x'))

	if got := m.View().Content; strings.Contains(got, "brew install dtach") {
		t.Errorf("the notice survived a keypress:\n%s", got)
	}
}

// A session with no holder configured must not look archivable-but-broken: the
// default is a no-op, like the tailer stops, because a machine without dtach
// has no held process and doing nothing is the right answer (#43).
func TestModel_archiveWithNoStopConfiguredStillArchives_issue43(t *testing.T) {
	terms, _ := fakeTerms(t)
	st := worktreeState()
	r := &recordArchive{State: st}
	d := baseDeps(st, terms)
	d.Archive, d.TailStop, d.RemoveWorktree = r.archive, r.stopTail, r.removeWorktree
	m := ui.NewModel(d) // no d.Stop
	openArchive(t, m, "s2")

	press(m, key('y'))

	if len(r.Archived) != 1 {
		t.Errorf("archived = %v, want the session archived despite no holder", r.Archived)
	}
}
