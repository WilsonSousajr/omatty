package ui_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/vcs"
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

var errDiskFull = errors.New("disk full")

// turnDiffText is what one turn did on top of sampleDiff's first file: it
// added c := 4. The full diff also has b's change and new.txt; this does not.
const turnDiffText = `diff --git a/internal/ui/model.go b/internal/ui/model.go
index 3333333..2222222 100644
--- a/internal/ui/model.go
+++ b/internal/ui/model.go
@@ -10,4 +10,5 @@ func (m *Model) onKey() {
 	a := 1
 	b := 3
+	c := 4
 	return
 }
`

func turnDiffParsed(t *testing.T) review.Diff {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader(turnDiffText))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// modelWithTurnDiff opens the review column on s1 with sampleDiff as the
// whole session and turnDiffText as the turn.
func modelWithTurnDiff(t *testing.T, tr *turnRecorder) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	rec := &diffRecorder{Diff: sampleDiffParsed(t)}
	tr.Diff = turnDiffParsed(t)
	d := baseDeps(twoProjectState(), terms)
	d.Diff, d.Turn = rec.fn, tr.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('d'))
	return m, fakes
}

func TestModel_tShowsOnlyThisTurn_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})

	pressAndSettle(m, key('t'))
	body := m.View().Content
	if !strings.Contains(body, "this turn") || strings.Contains(body, "fresh") || strings.Contains(body, "b := 2") {
		t.Errorf("the turn scope does not show only this turn:\n%s", body)
	}

	pressAndSettle(m, key('t'))
	body = m.View().Content
	if strings.Contains(body, "this turn") || !strings.Contains(body, "fresh") {
		t.Errorf("t again does not return to the whole session:\n%s", body)
	}
}

// A narrow column gives up title parts; "this turn" is never one of them,
// because a turn view that reads as the whole diff is this feature's worst
// failure.
func TestModel_thisTurnStaysInTheTitleAtANarrowWidth_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	pressAndSettle(m, key('t'))

	if !strings.Contains(m.View().Content, "this turn") {
		t.Errorf("the title lost \"this turn\" at 80 columns:\n%s", m.View().Content)
	}
}

func TestModel_noBaselineSaysSo_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m, _ := modelWithTurnDiff(t, tr)
	tr.Diff, tr.DiffErr = review.Diff{}, review.ErrNoTurn

	pressAndSettle(m, key('t'))

	body := m.View().Content
	if !strings.Contains(body, "no turn recorded yet") || strings.Contains(body, "no changes") {
		t.Errorf("a missing baseline is not explained:\n%s", body)
	}
}

// After a failed snapshot the ref still names the previous turn; showing
// that diff would present two turns as one.
func TestModel_aFailedSnapshotShowsInsteadOfTheLastTurn_issue311(t *testing.T) {
	tr := &turnRecorder{SnapErr: errDiskFull}
	m, _ := modelWithTurnDiff(t, tr)
	_, cmd := m.Update(hookPrompt("s1"))
	settle(m, cmd)

	pressAndSettle(m, key('t'))

	body := m.View().Content
	if !strings.Contains(body, "could not be taken") || strings.Contains(body, "c := 4") {
		t.Errorf("a failed baseline did not replace the turn diff:\n%s", body)
	}
}

// A failed snapshot's notice shows what git objected to (#351). The error
// arrives wrapped twice - review's context, then vcs's command line and path -
// and one row cut it at the column edge long before git's own words. Its
// innermost error is only "exit status 128", so the notice shows git's stderr,
// every line of it, wrapped to the 27-column review column, and points at the
// log for the rest.
func TestModel_aFailedSnapshotShowsWhatGitSaid_issue351(t *testing.T) {
	dir := "/Users/someone/projects/a-rather-long-repository-name"
	gitErr := &vcs.CommandError{
		Args:   []string{"add", "-A"},
		Dir:    dir,
		Stderr: "error: open(\"secret.pem\"): Permission denied\nfatal: adding files failed",
		Err:    errors.New("exit status 128"),
	}
	tr := &turnRecorder{SnapErr: fmt.Errorf("review: snapshotting session s1 in %q: %w", dir, gitErr)}
	m, _ := modelWithTurnDiff(t, tr)
	_, cmd := m.Update(hookPrompt("s1"))
	settle(m, cmd)

	pressAndSettle(m, key('t'))

	body := stripSGR(m.View().Content)
	for _, want := range []string{"Permission denied", "fatal: adding files failed", "full error in the log"} {
		if !strings.Contains(body, want) {
			t.Errorf("the notice does not show %q:\n%s", want, body)
		}
	}
}

// The whole-session diff's failure had the same weakness (#351): one row, cut
// at the column edge, before git's words. It now shows the same way the turn
// notices do.
func TestModel_aFailedDiffLoadShowsWhatGitSaid_issue351(t *testing.T) {
	m, _, rec := modelWithDiff(t)
	rec.Err = fmt.Errorf("review: diffing session s1 against main: %w", &vcs.CommandError{
		Args:   []string{"diff", "main"},
		Dir:    "/Users/someone/projects/a-rather-long-repository-name",
		Stderr: "fatal: bad revision 'main'",
		Err:    errors.New("exit status 128"),
	})

	leader(m, key('d'))

	body := stripSGR(m.View().Content)
	for _, want := range []string{"bad revision 'main'", "full error in the log"} {
		if !strings.Contains(body, want) {
			t.Errorf("the failed diff does not show %q:\n%s", want, body)
		}
	}
}

// Outside this turn is not moved - it is elsewhere in the session - so the
// comment is hidden here, and still counted and sent.
func TestModel_aCommentOutsideThisTurnIsHiddenNotMoved_issue311(t *testing.T) {
	m, fakes := modelWithTurnDiff(t, &turnRecorder{})
	down(m, 3)
	typeNote(m, "about b")

	pressAndSettle(m, key('t'))
	body := m.View().Content
	if strings.Contains(body, "(moved)") || strings.Contains(body, "about b") {
		t.Errorf("a comment outside this turn is drawn:\n%s", body)
	}

	press(m, shiftS)
	if len(fakes["s1"].Sent) != 1 || !strings.Contains(fakes["s1"].Sent[0], "about b") {
		t.Errorf("S in the turn scope did not send the comment outside it: %q", fakes["s1"].Sent)
	}
}

// Review Focus 3: the anchor is content, so a comment written on this
// turn's line sits on the same line in the whole-session view.
func TestModel_aCommentWrittenInTheTurnScopeStaysOnItsLine_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	pressAndSettle(m, key('t'))
	down(m, 4) // file, hunk, a, b, then +c := 4
	typeNote(m, "why 4")

	pressAndSettle(m, key('t'))

	body := m.View().Content
	if strings.Contains(body, "(moved) why 4") || !commentFollows(body, "c := 4", ">> why 4") {
		t.Errorf("the comment is not under c := 4 in the whole-session view:\n%s", body)
	}
}

// commentFollows reports whether the row carrying note is the one right after
// the row carrying line, in the review column's part of each screen row.
func commentFollows(body, line, note string) bool {
	rows := strings.Split(stripSGR(body), "\n")
	for i := 0; i+1 < len(rows); i++ {
		if strings.Contains(rows[i], line) && strings.Contains(rows[i+1], note) {
			return true
		}
	}
	return false
}

// PruneSent drops a sent comment whose line is gone. Outside this turn is
// not gone, so a turn load must never prune.
func TestModel_aTurnLoadNeverPrunesASentComment_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	down(m, 3)
	typeNote(m, "why?")
	press(m, shiftS)
	refocusColumn(m)

	pressAndSettle(m, key('t'))
	pressAndSettle(m, key('t'))

	if !strings.Contains(m.View().Content, "(sent") {
		t.Errorf("the sent comment was pruned by the turn load:\n%s", m.View().Content)
	}
}

// Review Focus 2: a turn diff that arrives after the column moved on, or
// after the scope went back to the whole session, is dropped.
func TestModel_aLateTurnDiffIsDropped_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})

	m.Update(ui.TurnLoadedMsg{SessionID: "s1", Diff: turnDiffParsed(t)})

	body := m.View().Content
	if strings.Contains(body, "this turn") || !strings.Contains(body, "fresh") {
		t.Errorf("a turn diff replaced the whole-session view:\n%s", body)
	}
}

// The column is on s1's turn; a turn diff for another session, answering a
// load from before the column moved, must not be drawn as s1's.
func TestModel_aTurnDiffForAnotherSessionIsDropped_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	pressAndSettle(m, key('t'))

	m.Update(ui.TurnLoadedMsg{SessionID: "s2", Diff: sampleDiffParsed(t)})

	if body := m.View().Content; strings.Contains(body, "fresh") {
		t.Errorf("s2's turn diff was drawn in s1's column:\n%s", body)
	}
}

// Review Focus 4.
func TestModel_rInTheTurnScopeReloadsTheTurn_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m, _ := modelWithTurnDiff(t, tr)
	pressAndSettle(m, key('t'))
	before := tr.DiffCalls

	pressAndSettle(m, key('r'))

	if tr.DiffCalls != before+1 {
		t.Errorf("r loaded the turn %d times, want 1", tr.DiffCalls-before)
	}
}

// Review Focus 5: before the turn diff arrives the column says it is
// reading, never "no changes", which would be a claim.
func TestModel_theTurnScopeSaysItIsReadingUntilTheDiffArrives_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})

	press(m, key('t')) // the load is scheduled, not yet run

	body := m.View().Content
	if !strings.Contains(body, "reading this turn") || strings.Contains(body, "no changes") {
		t.Errorf("the turn scope claims something before its diff arrived:\n%s", body)
	}
}

// A snapshot that lands while the turn view is open reloads it.
func TestModel_aSnapshotReloadsAnOpenTurnView_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m, _ := modelWithTurnDiff(t, tr)
	pressAndSettle(m, key('t'))
	before := tr.DiffCalls

	_, cmd := m.Update(hookPrompt("s1"))
	settle(m, cmd)

	if tr.DiffCalls != before+1 {
		t.Errorf("the turn view loaded %d times after a snapshot, want 1", tr.DiffCalls-before)
	}
}

// Final review, I1: two turns add a "}" each, and the turn's hunk header
// counts from its baseline, so it matches nothing in the session diff. A
// comment on this turn's "}" must reach claude as that line, f.go:5 - before
// the fix the anchor fell back to the first "+}" and claude was told f.go:3.
func TestModel_aTurnCommentOnARepeatedLineIsSentAsThatLine_issue311(t *testing.T) {
	session, err := review.ParseDiff(strings.NewReader("diff --git a/f.go b/f.go\nindex 1111111..2222222 100644\n--- a/f.go\n+++ b/f.go\n@@ -1 +1,5 @@\n package f\n+func a() {\n+}\n+func b() {\n+}\n"))
	if err != nil {
		t.Fatal(err)
	}
	turn, err := review.ParseDiff(strings.NewReader("diff --git a/f.go b/f.go\nindex 3333333..2222222 100644\n--- a/f.go\n+++ b/f.go\n@@ -1,3 +1,5 @@\n package f\n func a() {\n }\n+func b() {\n+}\n"))
	if err != nil {
		t.Fatal(err)
	}
	terms, fakes := fakeTerms(t)
	tr := &turnRecorder{Diff: turn}
	d := baseDeps(twoProjectState(), terms)
	d.Diff, d.Turn = (&diffRecorder{Diff: session}).fn, tr.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('d'))
	pressAndSettle(m, key('t'))

	down(m, 6) // file, hunk, package, func a, }, func b, then b's "+}"
	typeNote(m, "why")
	press(m, shiftS)

	if len(fakes["s1"].Sent) != 1 || !strings.Contains(fakes["s1"].Sent[0], "f.go:5") {
		t.Errorf("sent %q, want the comment located at f.go:5", fakes["s1"].Sent)
	}
}

// Final review, I2: a closed column keeps its content for the reopen (#124),
// but a prompt moves the baseline, so the cached turn diff is now the last
// turn's. Reopening must load the new one rather than show the old one
// labelled "this turn".
func TestModel_reopeningAfterAPromptLoadsTheNewTurn_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m, _ := modelWithTurnDiff(t, tr)
	pressAndSettle(m, key('t'))
	leader(m, key('d')) // close; the column keeps its turn view
	before := tr.DiffCalls

	_, cmd := m.Update(hookPrompt("s1"))
	settle(m, cmd)
	leader(m, key('d'))

	if tr.DiffCalls != before+1 {
		t.Errorf("reopening after a prompt loaded the turn %d times, want 1", tr.DiffCalls-before)
	}
}

// Final review, M5 (graded up): with the hook socket unbound (#49) no
// baseline will ever be taken, and a ref left from an earlier run would be
// diffed and called "this turn". The turn view says why instead, and loads
// nothing.
func TestModel_withHooksDownTheTurnViewSaysSoAndLoadsNothing_issue311(t *testing.T) {
	terms, _ := fakeTerms(t)
	tr := &turnRecorder{Diff: turnDiffParsed(t)}
	d := baseDeps(twoProjectState(), terms)
	d.Diff, d.Turn, d.HooksDown = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn, tr.funcs(), true
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('d'))

	pressAndSettle(m, key('t'))

	body := m.View().Content
	if !strings.Contains(body, "hooks are not") || strings.Contains(body, "c := 4") {
		t.Errorf("the turn view does not say hooks are down:\n%s", body)
	}
	if tr.DiffCalls != 0 {
		t.Errorf("loaded the turn %d times with hooks down, want 0", tr.DiffCalls)
	}
}
