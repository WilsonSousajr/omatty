package app_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// FakeFollow is a named app.FollowFunc fake: the checkout each directory
// resolves to, and the directories it was asked about. A directory it does
// not know resolves to the session unchanged, as another repository does.
type FakeFollow struct {
	To    map[string]session.Session // by cwd: the Dir and Branch to read
	Asked []string
}

func (f *FakeFollow) fn(sess session.Session, cwd string) (session.Session, error) {
	f.Asked = append(f.Asked, cwd)
	if to, ok := f.To[cwd]; ok {
		sess.Dir, sess.Branch = to.Dir, to.Branch
	}
	return sess, nil
}

// worktreeModel is the tree open on s1, with claude able to move into
// /p/omatty/.worktrees/x on feat-x.
func worktreeModel(t *testing.T) (*app.Model, *fileLister, *diffRecorder, *FakeFollow) {
	t.Helper()
	terms, _ := fakeTerms(t)
	lister := &fileLister{Paths: []string{"go.mod"}}
	rec := &diffRecorder{Diff: sampleDiffParsed(t)}
	follow := &FakeFollow{To: map[string]session.Session{
		"/p/omatty/.worktrees/x/internal": {Dir: "/p/omatty/.worktrees/x", Branch: "feat-x"},
	}}
	d := baseDeps(twoProjectState(), terms)
	d.Diff, d.Files, d.Follow = rec.fn, lister.fn, follow.fn
	m := app.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('f'))
	lister.Asked, rec.Dirs = nil, nil
	return m, lister, rec, follow
}

// reportStatus delivers one event for s1 the way the runtime would.
func reportStatus(m *app.Model, kind dstatus.Kind, at time.Time, cwd string) {
	_, cmd := m.Update(app.StatusMsg{SessionID: "s1", Kind: kind, At: at, Cwd: cwd})
	deliver(m, cmd)
}

// claude moved into a worktree mid-session and the tree stayed on the
// directory omatty launched it in (#659). At the turn's end the tree and the
// diff read the checkout claude is working in.
func TestReview_followsTheWorktreeClaudeMovedInto_issue659(t *testing.T) {
	m, lister, rec, _ := worktreeModel(t)
	at := time.Now()

	reportStatus(m, dstatus.PromptSubmitted, at, "/p/omatty/.worktrees/x/internal")
	reportStatus(m, dstatus.TurnEnded, at.Add(time.Second), "/p/omatty/.worktrees/x/internal")

	if len(lister.Asked) == 0 || lister.Asked[len(lister.Asked)-1] != "/p/omatty/.worktrees/x" {
		t.Errorf("tree listed %v, want the worktree last", lister.Asked)
	}
	if len(rec.Dirs) == 0 || rec.Dirs[len(rec.Dirs)-1] != "/p/omatty/.worktrees/x" {
		t.Errorf("diff read %v, want the worktree last", rec.Dirs)
	}
}

// Mid-turn the directory is only remembered: the move takes effect at the
// turn's end, so the brief returns to the launch directory inside a run do
// not make the tree jump (#659's findings).
func TestReview_aMoveMidTurnWaitsForTheTurnsEnd_issue659(t *testing.T) {
	m, _, _, follow := worktreeModel(t)
	at := time.Now()

	reportStatus(m, dstatus.PromptSubmitted, at, "/p/omatty/.worktrees/x/internal")
	reportStatus(m, dstatus.ToolStarted, at.Add(time.Second), "/p/omatty")
	reportStatus(m, dstatus.ToolFinished, at.Add(2*time.Second), "/p/omatty/.worktrees/x/internal")
	if len(follow.Asked) != 0 {
		t.Fatalf("asked about %v mid-turn, want nothing until the turn ends", follow.Asked)
	}
	reportStatus(m, dstatus.TurnEnded, at.Add(3*time.Second), "")

	if len(follow.Asked) != 1 || follow.Asked[0] != "/p/omatty/.worktrees/x/internal" {
		t.Errorf("asked about %v, want the last directory once at the turn's end", follow.Asked)
	}
}

// A directory that resolves to the session's own checkout - a subdirectory,
// or another repository the guard refuses - leaves the review where it was.
func TestReview_aDirectoryThatDoesNotMoveTheCheckoutChangesNothing_issue659(t *testing.T) {
	m, lister, _, _ := worktreeModel(t)
	at := time.Now()

	reportStatus(m, dstatus.PromptSubmitted, at, "/elsewhere")
	reportStatus(m, dstatus.TurnEnded, at.Add(time.Second), "/elsewhere")

	for _, dir := range lister.Asked {
		if dir != "" {
			t.Errorf("tree listed %q, want the session's own directory only", dir)
		}
	}
}

// The same directory is not resolved twice: git is asked once per move, not
// once per turn.
func TestReview_aDirectoryAlreadyFollowedIsNotAskedAgain_issue659(t *testing.T) {
	m, _, _, follow := worktreeModel(t)
	at := time.Now()

	for i := range 3 {
		step := at.Add(time.Duration(2*i) * time.Second)
		reportStatus(m, dstatus.PromptSubmitted, step, "/p/omatty/.worktrees/x/internal")
		reportStatus(m, dstatus.TurnEnded, step.Add(time.Second), "/p/omatty/.worktrees/x/internal")
	}

	if len(follow.Asked) != 1 {
		t.Errorf("asked %v, want one question for one move", follow.Asked)
	}
}

// A listing of the old checkout that lands after the review moved is not
// drawn; the new checkout is listed in its place (#659).
func TestReview_aListingOfTheOldCheckoutIsListedAgain_issue659(t *testing.T) {
	m, lister, _, _ := worktreeModel(t)
	_, cmd := m.Update(app.WorkTreeMsg{SessionID: "s1", Cwd: "/wt", Sess: session.Session{ID: "s1", Dir: "/wt"}})
	deliver(m, cmd)
	lister.Asked = nil

	_, cmd = m.Update(app.FilesLoadedMsg{SessionID: "s1", Dir: "", Paths: []string{"old.go"}})
	deliver(m, cmd)

	if len(lister.Asked) != 1 || lister.Asked[0] != "/wt" {
		t.Errorf("listed %v after a listing of the old checkout, want /wt once", lister.Asked)
	}
}

// Back in its own checkout, the review reads the launch directory again.
func TestReview_movingBackHomeReadsTheLaunchDirectory_issue659(t *testing.T) {
	m, _, rec, _ := worktreeModel(t)
	_, cmd := m.Update(app.WorkTreeMsg{SessionID: "s1", Cwd: "/wt", Sess: session.Session{ID: "s1", Dir: "/wt"}})
	deliver(m, cmd)

	_, cmd = m.Update(app.WorkTreeMsg{SessionID: "s1", Cwd: "/home", Sess: session.Session{ID: "s1", Dir: ""}})
	deliver(m, cmd)

	if len(rec.Dirs) == 0 || rec.Dirs[len(rec.Dirs)-1] != "" {
		t.Errorf("diff read %v, want the launch directory last", rec.Dirs)
	}
}

// A directory git cannot resolve leaves the review where it was.
func TestReview_aFollowThatFailsChangesNothing_issue659(t *testing.T) {
	m, lister, _, _ := worktreeModel(t)

	_, cmd := m.Update(app.WorkTreeMsg{SessionID: "s1", Cwd: "/gone", Err: errListing})
	deliver(m, cmd)

	if len(lister.Asked) != 0 {
		t.Errorf("listed %v after a failed follow, want nothing", lister.Asked)
	}
}
