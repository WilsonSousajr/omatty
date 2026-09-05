package ui_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// recordArchive is a named fake for the two archive dependencies plus the
// tailer stop, so one test can assert the whole teardown ran.
type recordArchive struct {
	Archived   []string
	Stopped    []string
	Removed    [][2]string // repoRoot, dir
	ArchiveErr error
	RemoveErr  error
	// State is what the registry holds. The real RemoveSession re-reads
	// state.json and returns the row it found there, so this fake reads from
	// its own copy - which a test can make disagree with the model's, since
	// that divergence is the whole reason the row is returned (#40).
	State registry.State
}

func (r *recordArchive) archive(sessionID string) (registry.Session, error) {
	r.Archived = append(r.Archived, sessionID)
	for _, sess := range r.State.Sessions {
		if sess.ID == sessionID {
			return sess, r.ArchiveErr
		}
	}
	return registry.Session{}, r.ArchiveErr
}

func (r *recordArchive) stopTail(sessionID string) { r.Stopped = append(r.Stopped, sessionID) }

func (r *recordArchive) removeWorktree(repoRoot, dir string) error {
	r.Removed = append(r.Removed, [2]string{repoRoot, dir})
	return r.RemoveErr
}

// worktreeState gives s2 a worktree, so the three-answer confirmation and the
// removal path are both reachable.
func worktreeState() registry.State {
	st := twoProjectState()
	for i := range st.Sessions {
		if st.Sessions[i].ID == "s2" {
			st.Sessions[i].Worktree = true
			st.Sessions[i].Dir = "/wt/omatty/parser-fix"
			st.Sessions[i].Branch = "parser-fix"
		}
	}
	return st
}

func modelWithArchive(t *testing.T, r *recordArchive) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	st := worktreeState()
	if len(r.State.Sessions) == 0 {
		r.State = st // the registry agrees with the model unless a test says otherwise
	}
	d := baseDeps(st, terms)
	d.Archive, d.TailStop, d.RemoveWorktree = r.archive, r.stopTail, r.removeWorktree
	return ui.NewModel(d), fakes
}

// Regression, issue #40: the worktree decision was made from the model's copy
// of the session, and RemoveSession's authoritative one - re-read from
// state.json - was discarded. Where the two disagree (a second omatty
// instance, a hand-edited state.json) omatty would run `git worktree remove
// --force` on a directory the registry no longer marks as a worktree.
func TestModel_archiveTrustsTheRegistrysRowNotItsOwn_issue40(t *testing.T) {
	r := &recordArchive{}
	m, _ := modelWithArchive(t, r)
	// The registry has since been told this session is not on a worktree; the
	// model still believes it is, which is what opened the `w` answer.
	for i := range r.State.Sessions {
		r.State.Sessions[i].Worktree = false
	}
	openArchive(m, "s2")

	press(m, key('w'))

	if len(r.Removed) != 0 {
		t.Errorf("removed = %v, want nothing: the registry says this is not a worktree", r.Removed)
	}
}

// openArchive puts the cursor on id and opens the confirmation over it.
func openArchive(m *ui.Model, id string) {
	for m.Focused() != id {
		press(m, ctrl('o'))
		press(m, key('j'))
	}
	press(m, ctrl('o'))
	press(m, key('x'))
}

// A main-checkout session must not offer to delete its directory: omatty did
// not create it.
func TestModel_archiveOffersNoWorktreeRemovalForAMainCheckout_issue40(t *testing.T) {
	m, _ := modelWithArchive(t, &recordArchive{})

	openArchive(m, "s1")

	got := m.View().Content
	if !strings.Contains(got, "archive session") {
		t.Fatalf("ctrl+o x did not open the confirmation:\n%s", got)
	}
	if strings.Contains(got, "[w]") {
		t.Errorf("a main-checkout session offered to remove a worktree:\n%s", got)
	}
}

// A worktree session gets the destructive answer on its own key, never on the
// one the hand reaches for.
func TestModel_archiveOffersWorktreeRemovalOnItsOwnKey_issue40(t *testing.T) {
	m, _ := modelWithArchive(t, &recordArchive{})

	openArchive(m, "s2")

	got := m.View().Content
	if !strings.Contains(got, "[y]") || !strings.Contains(got, "[w]") {
		t.Errorf("the worktree confirmation does not offer both answers:\n%s", got)
	}
	if !strings.Contains(got, "uncommitted") {
		t.Errorf("the destructive answer does not say what it discards:\n%s", got)
	}
}

func TestModel_archiveTearsDownEverythingTheSessionOwned_issue40(t *testing.T) {
	r := &recordArchive{}
	m, fakes := modelWithArchive(t, r)
	openArchive(m, "s1")

	press(m, key('y'))

	if len(r.Archived) != 1 || r.Archived[0] != "s1" {
		t.Errorf("archived = %v, want [s1]", r.Archived)
	}
	if len(r.Stopped) != 1 || r.Stopped[0] != "s1" {
		t.Errorf("tailers stopped = %v, want [s1]", r.Stopped)
	}
	if !fakes["s1"].Closed {
		t.Error("the archived session's terminal was not closed; its process would leak")
	}
	if fakes["s2"].Closed || fakes["s3"].Closed {
		t.Error("archiving one session closed another")
	}
	if got := m.View().Content; strings.Contains(got, "» ") && m.Focused() == "s1" {
		t.Errorf("the archived session is still selected:\n%s", got)
	}
}

// y keeps the worktree. That is the whole reason it is a separate answer.
func TestModel_archiveWithYKeepsTheWorktree_issue40(t *testing.T) {
	r := &recordArchive{}
	m, _ := modelWithArchive(t, r)
	openArchive(m, "s2")

	press(m, key('y'))

	if len(r.Removed) != 0 {
		t.Errorf("y removed worktrees %v, want none", r.Removed)
	}
}

func TestModel_archiveWithWRemovesTheWorktree_issue40(t *testing.T) {
	r := &recordArchive{}
	m, _ := modelWithArchive(t, r)
	openArchive(m, "s2")

	_, cmd := m.Update(key('w'))
	deliver(m, cmd)

	if len(r.Removed) != 1 || r.Removed[0] != [2]string{"/p/omatty", "/wt/omatty/parser-fix"} {
		t.Errorf("removed = %v, want one call with the project root and the worktree dir", r.Removed)
	}
}

// After a kill the cursor can land anywhere, because SetRows falls back to the
// first session row rather than the neighbour. Whatever it lands on must be
// sized, or claude paints at the wrong width there (#73, #95).
func TestModel_archiveSizesWhicheverSessionTheCursorLandsOn_issue40(t *testing.T) {
	r := &recordArchive{}
	m, fakes := modelWithArchive(t, r)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	openArchive(m, "s1")

	press(m, key('y'))

	landed := m.Focused()
	if landed == "" || landed == "s1" {
		t.Fatalf("cursor is on %q after archiving s1, want another session", landed)
	}
	wantW, wantH := ui.PTYSize(120, 40, false)
	if f := fakes[landed]; f.Width != wantW || f.Height != wantH {
		t.Errorf("session %s is %dx%d after landing on it, want %dx%d",
			landed, f.Width, f.Height, wantW, wantH)
	}
}

// A failed registry edit must leave the session running and on screen: half an
// archive is worse than none.
func TestModel_archiveFailureKeepsTheSessionAlive_issue40(t *testing.T) {
	r := &recordArchive{ArchiveErr: errors.New("state.json is read-only")}
	m, fakes := modelWithArchive(t, r)
	openArchive(m, "s1")

	press(m, key('y'))

	if fakes["s1"].Closed {
		t.Error("the terminal was closed even though the registry edit failed")
	}
	if len(r.Stopped) != 0 {
		t.Errorf("the tailer was stopped after a failed archive: %v", r.Stopped)
	}
	got := m.View().Content
	if !strings.Contains(got, "read-only") {
		t.Errorf("View() does not surface the failure:\n%s", got)
	}
	if !strings.Contains(got, "main") {
		t.Errorf("the session vanished from the sidebar after a failed archive:\n%s", got)
	}
}

// The session is already out of the registry by the time a removal fails, so
// the operator is told which directory is still on disk.
func TestModel_worktreeRemovalFailureNamesTheDirectoryLeftBehind_issue40(t *testing.T) {
	m, _ := modelWithArchive(t, &recordArchive{})

	m.Update(ui.WorktreeRemovedMsg{
		SessionID: "s2", Dir: "/wt/omatty/parser-fix", Err: errors.New("contains modified files"),
	})

	if got := m.View().Content; !strings.Contains(got, "/wt/omatty/parser-fix") {
		t.Errorf("View() does not name the worktree left on disk:\n%s", got)
	}
}

func TestModel_archiveEscCancels_issue40(t *testing.T) {
	r := &recordArchive{}
	m, fakes := modelWithArchive(t, r)
	openArchive(m, "s1")

	press(m, special(tea.KeyEscape))

	if len(r.Archived) != 0 {
		t.Errorf("esc archived %v, want nothing", r.Archived)
	}
	if fakes["s1"].Closed {
		t.Error("esc closed the terminal")
	}
}

// A key that is not an offered answer must not dismiss the question: the
// operator meant to answer it.
func TestModel_archiveIgnoresAnUnofferedKey_issue40(t *testing.T) {
	r := &recordArchive{}
	m, _ := modelWithArchive(t, r)
	openArchive(m, "s1")

	press(m, key('z'))

	if len(r.Archived) != 0 {
		t.Errorf("an unoffered key archived %v, want nothing", r.Archived)
	}
	if got := m.View().Content; !strings.Contains(got, "archive session") {
		t.Errorf("an unoffered key dismissed the confirmation:\n%s", got)
	}
}

// w on a main-checkout session is not an offered answer, so it must do
// nothing at all - never delete a directory omatty did not create.
func TestModel_archiveWOnAMainCheckoutDoesNothing_issue40(t *testing.T) {
	r := &recordArchive{}
	m, _ := modelWithArchive(t, r)
	openArchive(m, "s1")

	press(m, key('w'))

	if len(r.Removed) != 0 || len(r.Archived) != 0 {
		t.Errorf("w on a main checkout removed %v and archived %v, want neither", r.Removed, r.Archived)
	}
}

// Issue #28, for the confirmation: an open surface must never trap the
// operator.
func TestModel_ctrlCQuitsWhileTheConfirmationIsOpen_issue28(t *testing.T) {
	m, _ := modelWithArchive(t, &recordArchive{})
	openArchive(m, "s1")

	_, cmd := m.Update(ctrl('c'))

	if !isQuit(cmd) {
		t.Error("ctrl+c while the archive confirmation is open did not quit")
	}
}

// Invariant 1: the confirmation's keys are answers, not input for Claude.
func TestModel_confirmKeysStayOutOfThePTY_issue40(t *testing.T) {
	m, fakes := modelWithArchive(t, &recordArchive{})
	before := len(fakes["s1"].Msgs)
	openArchive(m, "s1")

	press(m, key('z'))

	if got := len(fakes["s1"].Msgs); got != before {
		t.Errorf("the terminal received %d messages while the confirmation was open, want %d",
			got, before)
	}
}
