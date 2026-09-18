package ui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// recordBranchRename is a named BranchRenameFunc fake: it records the rename
// and answers what the test set for "did the branch have commits of its own".
type recordBranchRename struct {
	Session  registry.Session
	Branch   string
	Calls    int
	Declined bool
	Forced   bool
	Err      error
}

func (r *recordBranchRename) fn(sess registry.Session, branch string, unstartedOnly bool) (bool, error) {
	r.Calls++
	r.Session, r.Branch, r.Forced = sess, branch, !unstartedOnly
	if r.Err != nil {
		return false, r.Err
	}
	return !r.Declined, nil
}

// ctrl+o N was the one prompt that still demanded a string, because
// `git worktree add -b` bakes the name in at creation. It no longer does: an
// empty buffer registers the worktree under a placeholder (#151).
func TestModel_aWorktreePromptNoLongerDemandsAName_issue151(t *testing.T) {
	c := &recordCreate{}
	m, _ := modelWithCreate(t, c)

	press(m, ctrl('o'))
	press(m, tea.KeyPressMsg{Code: 'N', Mod: tea.ModShift, Text: "N"})
	press(m, special(tea.KeyEnter))

	if c.Calls != 1 {
		t.Fatalf("create was called %d times on an empty worktree prompt, want 1", c.Calls)
	}
	if c.Branch != "" {
		t.Errorf("branch = %q, want empty so the registry names it", c.Branch)
	}
	if !c.Worktree {
		t.Error("the session was not created on a worktree, which is what N asked for")
	}
	if m.Prompt().Active {
		t.Error("the prompt is still open, so it is still demanding a name")
	}
}

// A name typed at creation still wins: #151 stops the prompt being compulsory,
// it does not take the choice away.
func TestModel_aTypedWorktreeNameIsStillUsed_issue151(t *testing.T) {
	c := &recordCreate{}
	m, _ := modelWithCreate(t, c)

	press(m, ctrl('o'))
	press(m, tea.KeyPressMsg{Code: 'N', Mod: tea.ModShift, Text: "N"})
	for _, r := range "fix" {
		press(m, key(r))
	}
	press(m, special(tea.KeyEnter))

	if c.Branch != "fix" || c.Title != "fix" {
		t.Errorf("create(title %q, branch %q), want both %q", c.Title, c.Branch, "fix")
	}
}

// Once the first prompt has said what the work is, the placeholder branch
// takes that name - the slug of it, the same filter model output passes.
func TestModel_theFirstPromptRenamesThePlaceholderBranch_issue151(t *testing.T) {
	r := &recordBranchRename{}
	m := placeholderWorktree(t, r, "Fix the horizontal wheel pan!")

	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if r.Calls != 1 {
		t.Fatalf("the branch rename ran %d times, want 1", r.Calls)
	}
	if r.Branch != "fix-the-horizontal-wheel-pan" {
		t.Errorf("branch = %q, want the slug of the first prompt", r.Branch)
	}
}

// A branch the operator named at creation is theirs; only a placeholder is
// renamed out from under them.
func TestModel_aNamedBranchIsNeverRenamed_issue151(t *testing.T) {
	r := &recordBranchRename{}
	m := modelForBranchRename(t, r, "Fix the wheel", "parser-fix", true)

	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if r.Calls != 0 {
		t.Errorf("the branch rename ran %d times against a named branch, want 0", r.Calls)
	}
}

// A main-checkout session has no branch of omatty's to rename.
func TestModel_aMainCheckoutSessionHasNoBranchToRename_issue151(t *testing.T) {
	r := &recordBranchRename{}
	m := modelForBranchRename(t, r, "Fix the wheel pan", "", false)

	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if r.Calls != 0 {
		t.Errorf("the branch rename ran %d times for a main-checkout session, want 0", r.Calls)
	}
}

// A failure leaves the placeholder in place and says so in the footer: naming
// never blocks the session, and a branch that quietly did not change would be
// worse than one that says it did not.
func TestModel_aFailedBranchRenameKeepsThePlaceholderAndSaysSo_issue151(t *testing.T) {
	r := &recordBranchRename{Err: errors.New("boom")}
	m := placeholderWorktree(t, r, "Fix the wheel pan")

	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if got := m.View().Content; !strings.Contains(got, "boom") {
		t.Errorf("nothing in the footer says the branch rename failed:\n%s", got)
	}
	if m.SessionBranch("s1") != registry.PlaceholderBranch("s1") {
		t.Errorf("branch = %q, want the placeholder kept", m.SessionBranch("s1"))
	}
}

// A branch with commits of its own keeps its name: it is in a history someone
// may already have pushed, and renaming it is the operator's call.
func TestModel_aBranchWithCommitsKeepsItsPlaceholder_issue151(t *testing.T) {
	r := &recordBranchRename{Declined: true}
	m := placeholderWorktree(t, r, "Fix the wheel pan")

	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if m.SessionBranch("s1") != registry.PlaceholderBranch("s1") {
		t.Errorf("branch = %q, want the placeholder kept when declined", m.SessionBranch("s1"))
	}
	if got := m.View().Content; strings.Contains(got, "error") {
		t.Errorf("declining to rename was reported as a failure:\n%s", got)
	}
}

// ctrl+o B renames a branch by hand, so a placeholder that outlived its first
// prompt is never permanent.
func TestModel_leaderBRenamesTheBranch_issue151(t *testing.T) {
	r := &recordBranchRename{}
	m := placeholderWorktree(t, r, "")

	leader(m, tea.KeyPressMsg{Code: 'B', Mod: tea.ModShift, Text: "B"})
	// Pre-filled with the branch it is on, as ctrl+o R is with the title.
	for range len(registry.PlaceholderBranch("s1")) {
		press(m, special(tea.KeyBackspace))
	}
	for _, c := range "my branch" {
		press(m, key(c))
	}
	pressAndSettle(m, special(tea.KeyEnter))

	if r.Branch != "my-branch" {
		t.Errorf("branch = %q, want the typed name slugged", r.Branch)
	}
	if !r.Forced {
		t.Error("a rename the operator asked for was still held to the no-commits rule")
	}
}

// modelForBranchRename is a one-session model whose session is about to be
// named by its first prompt, on a worktree or on the main checkout.
func modelForBranchRename(t *testing.T, r *recordBranchRename, prompt, branch string, worktree bool) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	st := twoProjectState()
	st.Sessions[0].Title = registry.PlaceholderTitle("s1")
	st.Sessions[0].Worktree, st.Sessions[0].Branch = worktree, branch
	d := baseDeps(st, terms)
	d.Name = (&FakeNamer{Titles: map[string]string{"s1": prompt}}).Name
	d.Rename = (&FakeRename{}).Rename
	d.RenameBranch = r.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

// placeholderWorktree is the case #151 is about: a worktree omatty named
// itself, waiting for its first prompt to say what the work is.
func placeholderWorktree(t *testing.T, r *recordBranchRename, prompt string) *ui.Model {
	t.Helper()
	return modelForBranchRename(t, r, prompt, registry.PlaceholderBranch("s1"), true)
}

// The branch box keeps the guard the worktree prompt gave up: a blank buffer
// would blank a branch that exists, which is the half of #127's rule #151
// leaves standing.
func TestModel_theBranchBoxStillRefusesABlankName_issue151(t *testing.T) {
	r := &recordBranchRename{}
	m := placeholderWorktree(t, r, "")

	leader(m, tea.KeyPressMsg{Code: 'B', Mod: tea.ModShift, Text: "B"})
	for range len(registry.PlaceholderBranch("s1")) {
		press(m, special(tea.KeyBackspace))
	}
	pressAndSettle(m, special(tea.KeyEnter))

	if r.Calls != 0 {
		t.Errorf("a blank branch box renamed anyway (%d calls), want it left open", r.Calls)
	}
}
