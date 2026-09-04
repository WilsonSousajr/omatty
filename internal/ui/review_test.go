package ui_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// sampleDiff mirrors review's fixture: one modified file, one new file.
const sampleDiff = `diff --git a/internal/ui/model.go b/internal/ui/model.go
index 1111111..2222222 100644
--- a/internal/ui/model.go
+++ b/internal/ui/model.go
@@ -10,4 +10,5 @@ func (m *Model) onKey() {
 	a := 1
-	b := 2
+	b := 3
+	c := 4
 	return
 }
diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+fresh
+file
`

// diffRecorder is a named DiffFunc fake: it serves one diff and records the
// sessions it was asked about.
type diffRecorder struct {
	Diff  review.Diff
	Err   error
	Asked []string
	Roots []string
}

func (r *diffRecorder) fn(sess registry.Session, root string) (review.Diff, error) {
	r.Asked = append(r.Asked, sess.ID)
	r.Roots = append(r.Roots, root)
	return r.Diff, r.Err
}

func sampleDiffParsed(t *testing.T) review.Diff {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader(sampleDiff))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// deliver runs a command tree synchronously and feeds every message it
// produces back into the model, so a diff loaded off the event loop lands
// before the assertion. Never call it on a blocking command (a tick or an
// event wait): these models have no event channel.
func deliver(m *ui.Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			deliver(m, c)
		}
		return
	}
	if msg != nil {
		_, next := m.Update(msg)
		deliver(m, next)
	}
}

func modelWithDiff(t *testing.T) (*ui.Model, map[string]*termwrap.Fake, *diffRecorder) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	rec := &diffRecorder{Diff: sampleDiffParsed(t)}
	d := baseDeps(twoProjectState(), terms)
	d.Diff = rec.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m, fakes, rec
}

// leader presses ctrl+o then k, delivering what the command returns.
func leader(m *ui.Model, k tea.KeyPressMsg) {
	press(m, ctrl('o'))
	_, cmd := m.Update(k)
	deliver(m, cmd)
}

func TestModel_LeaderDOpensTheReviewOnTheFocusedSession_issue21(t *testing.T) {
	m, fakes, rec := modelWithDiff(t)

	leader(m, key('d'))

	if !m.ReviewOpen() || !m.ReviewFocused() {
		t.Fatalf("after ctrl+o d: open=%v focused=%v, want both true", m.ReviewOpen(), m.ReviewFocused())
	}
	if len(rec.Asked) != 1 || rec.Asked[0] != "s1" || rec.Roots[0] != "/p/omatty" {
		t.Errorf("diff asked for %v in %v, want s1 in /p/omatty", rec.Asked, rec.Roots)
	}
	if !strings.Contains(m.View().Content, "internal/ui/model.go") {
		t.Errorf("the loaded diff is not rendered:\n%s", m.View().Content)
	}
	if fakes["s1"].Width != 42 {
		t.Errorf("terminal width = %d after opening, want 42", fakes["s1"].Width)
	}
}

func TestModel_LeaderDAgainClosesTheReviewAndRestoresTheTerminal_issue21(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))

	leader(m, key('d'))

	if m.ReviewOpen() {
		t.Fatal("review still open after a second ctrl+o d")
	}
	if fakes["s1"].Width != 70 {
		t.Errorf("terminal width = %d after closing, want 70 (PaneSize 100 closed)", fakes["s1"].Width)
	}
}

// Invariant 1 still holds with the pane focused: the leader is the only key
// omatty takes, and plain keys go to the pane rather than the PTY.
func TestModel_PlainKeysStayOutOfThePTYWhileTheReviewHasFocus_issue21(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	before := len(fakes["s1"].Msgs)

	press(m, key('j'))
	press(m, special(tea.KeyEscape))

	if len(fakes["s1"].Msgs) != before {
		t.Errorf("terminal received %d keys while the review was focused, want 0",
			len(fakes["s1"].Msgs)-before)
	}
	if m.ReviewFocused() {
		t.Error("esc did not hand focus back to the terminal")
	}
	press(m, key('j'))
	if len(fakes["s1"].Msgs) != before+1 {
		t.Error("after esc, j did not reach the terminal")
	}
}

func TestModel_SwitchingSessionMovesAnOpenReview_issue21(t *testing.T) {
	m, _, rec := modelWithDiff(t)
	leader(m, key('d'))

	leader(m, key('j'))

	if strings.Join(rec.Asked, ",") != "s1,s2" {
		t.Errorf("diff asked for %v, want s1 then s2", rec.Asked)
	}
}

func TestModel_DiffErrorShowsInThePaneNotTheLog_issue21(t *testing.T) {
	m, _, rec := modelWithDiff(t)
	rec.Err = errors.New("git exploded")

	leader(m, key('d'))

	if !strings.Contains(m.View().Content, "git exploded") {
		t.Errorf("pane does not show the load error:\n%s", m.View().Content)
	}
}

// A load that finishes after the pane closed, or after the cursor moved to
// another session, must not paint a stale diff.
func TestModel_StaleDiffLoadIsDropped_issue21(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	leader(m, key('d'))

	m.Update(ui.DiffLoadedMsg{SessionID: "s1", Diff: sampleDiffParsed(t)})

	if m.ReviewOpen() || strings.Contains(m.View().Content, "internal/ui/model.go") {
		t.Error("a stale DiffLoadedMsg reopened or repainted the review")
	}
}

func TestModel_LeaderDWithNoSessionDoesNothing_issue21(t *testing.T) {
	terms, _ := fakeTerms(t)
	m := ui.NewModel(baseDeps(emptyState(), terms))

	leader(m, key('d'))

	if m.ReviewOpen() {
		t.Error("review opened with no session to review")
	}
}

// A model built without a Diff dependency must say so rather than show an
// empty diff, which would read as "this session changed nothing".
func TestModel_WithoutADiffSourceThePaneExplains_issue21(t *testing.T) {
	terms, _ := fakeTerms(t)
	m := ui.NewModel(baseDeps(twoProjectState(), terms))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	leader(m, key('d'))

	if !strings.Contains(m.View().Content, "no diff source") {
		t.Errorf("pane does not explain the missing dependency:\n%s", m.View().Content)
	}
}

// The moment the operator looks at a diff is when claude stops, so a turn
// ending or a permission prompt reloads an open review - but only for the
// session the column is showing, and only on a real state change.
func TestModel_OpenReviewReloadsWhenItsSessionStops_issue21(t *testing.T) {
	m, _, rec := modelWithDiff(t)
	leader(m, key('d'))

	for _, ev := range []ui.StatusMsg{
		{SessionID: "s1", Kind: watcher.TurnEnded, At: fixedNow},
		{SessionID: "s2", Kind: watcher.TurnEnded, At: fixedNow},
		{SessionID: "s1", Kind: watcher.UsageUpdated, At: fixedNow},
	} {
		_, cmd := m.Update(ev)
		deliver(m, cmd)
	}

	if strings.Join(rec.Asked, ",") != "s1,s1" {
		t.Errorf("diff loaded for %v, want s1 on open and s1 on its own turn end only", rec.Asked)
	}
}

// Regression, issue #90: esc leaves the column open but unfocused, and the
// leader key that opened it took the close branch, so stepping back into a
// pane still on screen cost two presses and reloaded the diff.
func TestModel_LeaderRefocusesAnOpenColumnBeforeClosingIt_issue90(t *testing.T) {
	m, _, rec := modelWithDiff(t)
	leader(m, key('d'))
	press(m, special(tea.KeyEscape))
	if !m.ReviewOpen() || m.ReviewFocused() {
		t.Fatalf("after esc: open=%v focused=%v, want open and unfocused",
			m.ReviewOpen(), m.ReviewFocused())
	}

	leader(m, key('d'))

	if !m.ReviewOpen() || !m.ReviewFocused() {
		t.Fatalf("ctrl+o d on an unfocused column: open=%v focused=%v, want it focused",
			m.ReviewOpen(), m.ReviewFocused())
	}
	if len(rec.Asked) != 1 {
		t.Errorf("the diff was loaded %d times, want 1: refocusing is not a reload", len(rec.Asked))
	}
	leader(m, key('d'))
	if m.ReviewOpen() {
		t.Error("ctrl+o d on a focused column should close it")
	}
}
