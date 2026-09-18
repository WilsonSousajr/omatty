package registry_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// #151 stops ctrl+o N demanding a branch name. A session created without one
// gets a placeholder derived from its uuid, and the placeholder must survive
// the same filter a typed name does - it reaches `git worktree add -b` and
// the filesystem.
func TestPlaceholderBranch_isSlugSafeAndDerivedFromTheID_issue151(t *testing.T) {
	const id = "A1B2C3D4-5678-4abc-9def-000000000000"
	got := registry.PlaceholderBranch(id)

	if want := "omatty-a1b2c3d4"; got != want {
		t.Errorf("PlaceholderBranch(%q) = %q, want %q", id, got, want)
	}
	if registry.Slug(got) != got {
		t.Errorf("PlaceholderBranch is not slug-safe: Slug(%q) = %q", got, registry.Slug(got))
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`).MatchString(got) {
		t.Errorf("PlaceholderBranch(%q) = %q, which is not a legal branch name", id, got)
	}
}

// Derived from the session, never tracked: a map is empty after a relaunch,
// so a session whose branch is still a placeholder has to be recognisable
// from state.json alone - the argument isPlaceholderTitle already makes.
func TestPlaceholderBranch_isRecomputableFromTheSessionAlone_issue151(t *testing.T) {
	const id = "abc12345-6789-4abc-9def-000000000000"

	first, again := registry.PlaceholderBranch(id), registry.PlaceholderBranch(id)
	if first != again {
		t.Errorf("PlaceholderBranch is not stable for one id: %q then %q", first, again)
	}
	if first == registry.PlaceholderBranch("def67890-6789-4abc-9def-000000000000") {
		t.Error("two sessions share a placeholder branch, so two worktrees would collide")
	}
}

// A branch the operator typed reaches `git worktree add -b` and a directory
// name, so it passes the same filter model output does - never a looser one
// (#127 step 2). Until #151 it was only trimmed.
func TestCreate_slugsATypedBranch_issue151(t *testing.T) {
	g := &FakeGit{}
	c := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"}, stubID)

	sess, err := c.Create(baseState(), "omatty", "", "../../etc/passwd")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if sess.Branch != "etc-passwd" {
		t.Errorf("branch = %q, want it slugged to %q", sess.Branch, "etc-passwd")
	}
	if strings.Contains(g.AddedDir, "..") {
		t.Errorf("worktree directory %q contains a traversal", g.AddedDir)
	}
}

// With no branch typed, the session still gets a worktree - the placeholder
// names it, and the first prompt renames it later.
func TestCreate_aWorktreeWithNoTypedBranchTakesThePlaceholder_issue151(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	c := registry.NewCreator(&FakeGit{}, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"},
		func() string { return id })

	sess, err := c.CreateWorktree(baseState(), "omatty", "", "")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v, want nil", err)
	}

	if want := registry.PlaceholderBranch(id); sess.Branch != want {
		t.Errorf("branch = %q, want the placeholder %q", sess.Branch, want)
	}
	if !sess.Worktree {
		t.Error("the session is not on a worktree, so nothing was created for it")
	}
}

// The rename writes Branch and nothing else. Dir stays where it is: claude is
// running in that directory and the transcript path is derived from it (#60).
func TestRenameBranch_writesTheBranchAndLeavesTheDirectory_issue151(t *testing.T) {
	store, _ := newStoreAt(t)
	st := registry.State{
		Version:  registry.Version,
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}},
		Sessions: []registry.Session{{
			ID: "s1", Project: "omatty", Title: "one",
			Dir: "/home/u/.omatty/wt/omatty/omatty-s1", Branch: "omatty-s1", Worktree: true,
		}},
	}
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}

	if err := registry.RenameBranch(store, "s1", "fix-the-wheel-pan"); err != nil {
		t.Fatalf("RenameBranch() error = %v, want nil", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Sessions[0].Branch != "fix-the-wheel-pan" {
		t.Errorf("branch = %q, want the new name", got.Sessions[0].Branch)
	}
	if got.Sessions[0].Dir != st.Sessions[0].Dir {
		t.Errorf("Dir moved to %q, want it left at %q", got.Sessions[0].Dir, st.Sessions[0].Dir)
	}
}

// AddWorktreeSession is what ctrl+o N reaches now: it registers the session,
// names the branch when nobody else did, and persists both.
func TestAddWorktreeSession_namesTheBranchAndPersistsIt_issue151(t *testing.T) {
	store, _ := newStoreAt(t)
	if err := store.Save(registry.State{
		Version:  registry.Version,
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}},
	}); err != nil {
		t.Fatal(err)
	}
	const id = "11111111-2222-4333-8444-555555555555"
	c := registry.NewCreator(&FakeGit{}, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"},
		func() string { return id })

	sess, err := registry.AddWorktreeSession(store, c, "omatty", "", "")
	if err != nil {
		t.Fatalf("AddWorktreeSession() error = %v, want nil", err)
	}

	if want := registry.PlaceholderBranch(id); sess.Branch != want {
		t.Errorf("branch = %q, want the placeholder %q", sess.Branch, want)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sessions) != 1 || got.Sessions[0].Branch != sess.Branch {
		t.Errorf("state.json holds %+v, want the session with its branch", got.Sessions)
	}
}

// A project that is not registered is refused before anything is created.
func TestAddWorktreeSession_refusesAnUnknownProject_issue151(t *testing.T) {
	store, _ := newStoreAt(t)
	c := registry.NewCreator(&FakeGit{}, registry.CreatorOpts{WorktreeRoot: "/wt"}, stubID)

	if _, err := registry.AddWorktreeSession(store, c, "nope", "", ""); err == nil {
		t.Error("AddWorktreeSession() into an unregistered project returned nil, want an error")
	}
}

// fakeBranchGit is a named BranchRenamer fake: it records the rename and
// answers what the test set for "has this branch commits of its own".
type fakeBranchGit struct {
	Root     string
	Commits  int
	From, To string
	Err      error
}

func (f *fakeBranchGit) MainCheckout(string) (string, error) { return f.Root, nil }
func (f *fakeBranchGit) CommitsOnBranch(string, string, string) (int, error) {
	return f.Commits, nil
}
func (f *fakeBranchGit) RenameBranch(_, old, name string) error {
	f.From, f.To = old, name
	return f.Err
}

func worktreeSessionStore(t *testing.T) (*registry.Store, registry.Session) {
	t.Helper()
	store, _ := newStoreAt(t)
	sess := registry.Session{
		ID: "s1", Project: "omatty", Title: "one",
		Dir: "/wt/omatty/omatty-s1", Branch: "omatty-s1", Base: "main", Worktree: true,
	}
	if err := store.Save(registry.State{
		Version:  registry.Version,
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}},
		Sessions: []registry.Session{sess},
	}); err != nil {
		t.Fatal(err)
	}
	return store, sess
}

// The automatic rename: git first, state.json second, and only while the
// branch has nothing committed to it.
func TestRenameSessionBranch_renamesAnUnstartedBranch_issue151(t *testing.T) {
	store, sess := worktreeSessionStore(t)
	g := &fakeBranchGit{Root: "/p/omatty"}

	renamed, err := registry.RenameSessionBranch(store, g, sess, "fix-the-wheel-pan", true)

	if err != nil || !renamed {
		t.Fatalf("RenameSessionBranch() = %v, %v, want true, nil", renamed, err)
	}
	if g.From != "omatty-s1" || g.To != "fix-the-wheel-pan" {
		t.Errorf("git renamed %q -> %q, want omatty-s1 -> fix-the-wheel-pan", g.From, g.To)
	}
	got, _ := store.Load()
	if got.Sessions[0].Branch != "fix-the-wheel-pan" {
		t.Errorf("state.json holds branch %q, want the new name", got.Sessions[0].Branch)
	}
}

// A branch with commits declines: false with a nil error, and git is not asked
// to rename anything.
func TestRenameSessionBranch_declinesAStartedBranch_issue151(t *testing.T) {
	store, sess := worktreeSessionStore(t)
	g := &fakeBranchGit{Root: "/p/omatty", Commits: 1}

	renamed, err := registry.RenameSessionBranch(store, g, sess, "fix-the-wheel-pan", true)

	if err != nil || renamed {
		t.Fatalf("RenameSessionBranch() = %v, %v, want false, nil", renamed, err)
	}
	if g.To != "" {
		t.Errorf("git was asked to rename to %q, want no call", g.To)
	}
}

// The operator's own ctrl+o B renames a started branch: the history is theirs.
func TestRenameSessionBranch_theOperatorMayRenameAStartedBranch_issue151(t *testing.T) {
	store, sess := worktreeSessionStore(t)
	g := &fakeBranchGit{Root: "/p/omatty", Commits: 3}

	renamed, err := registry.RenameSessionBranch(store, g, sess, "mine", false)

	if err != nil || !renamed {
		t.Fatalf("RenameSessionBranch() = %v, %v, want true, nil", renamed, err)
	}
	if g.To != "mine" {
		t.Errorf("git renamed to %q, want mine", g.To)
	}
}

// A session with no recorded base has nothing to count against, so it is
// treated as started and left alone - the safe answer for a row written before
// #21 (invariant 9).
func TestRenameSessionBranch_aSessionWithNoBaseIsLeftAlone_issue151(t *testing.T) {
	store, sess := worktreeSessionStore(t)
	sess.Base = ""
	g := &fakeBranchGit{Root: "/p/omatty"}

	renamed, err := registry.RenameSessionBranch(store, g, sess, "fix", true)

	if err != nil || renamed {
		t.Fatalf("RenameSessionBranch() = %v, %v, want false, nil", renamed, err)
	}
}
