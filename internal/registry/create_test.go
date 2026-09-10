package registry_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

func stubID() string { return "fixed-uuid" }

func baseState() *registry.State {
	return &registry.State{
		Version:  registry.Version,
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}},
	}
}

func TestCreator_OnMainCheckoutMakesNoWorktree(t *testing.T) {
	g := &FakeGit{}
	st := baseState()

	got, err := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"}, stubID).Create(st, "omatty", "poke", "")

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if got.Worktree {
		t.Error("Worktree = true, want false for a main-checkout session")
	}
	if got.Dir != "/p/omatty" {
		t.Errorf("Dir = %q, want the project root %q", got.Dir, "/p/omatty")
	}
	if g.AddedDir != "" {
		t.Errorf("AddWorktree was called with %q, want no call", g.AddedDir)
	}
	if len(st.Sessions) != 1 {
		t.Errorf("state holds %d sessions, want 1", len(st.Sessions))
	}
}

func TestCreator_OnBranchCreatesWorktree(t *testing.T) {
	g := &FakeGit{}

	got, err := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"}, stubID).
		Create(baseState(), "omatty", "parser", "parser-fix")

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	want := "/home/u/.omatty/wt/omatty/parser-fix"
	if got.Dir != want {
		t.Errorf("Dir = %q, want %q", got.Dir, want)
	}
	if !got.Worktree {
		t.Error("Worktree = false, want true")
	}
	if g.AddedRoot != "/p/omatty" || g.AddedDir != want {
		t.Errorf("AddWorktree(%q, %q), want (%q, %q)", g.AddedRoot, g.AddedDir, "/p/omatty", want)
	}
}

// #21: the review diff needs the branch a worktree was forked from, so the
// creator records it and forks from it explicitly.
func TestCreator_RecordsTheBaseBranchAndForksFromIt_issue21(t *testing.T) {
	g := &FakeGit{Branch: "develop"}

	got, err := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"}, stubID).
		Create(baseState(), "omatty", "parser", "parser-fix")

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if got.Base != "develop" {
		t.Errorf("Base = %q, want the root's branch %q", got.Base, "develop")
	}
	if g.AddedFrom != "develop" {
		t.Errorf("worktree forked from %q, want %q", g.AddedFrom, "develop")
	}
}

// A detached root reports "HEAD"; recording that would make review compare
// the worktree with itself, so it stays empty and review falls back.
func TestCreator_DetachedRootLeavesBaseEmpty_issue21(t *testing.T) {
	g := &FakeGit{Branch: "HEAD"}

	got, err := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"}, stubID).
		Create(baseState(), "omatty", "t", "b")

	if err != nil {
		t.Fatal(err)
	}
	if got.Base != "" {
		t.Errorf("Base = %q, want empty for a detached root", got.Base)
	}
}

func TestCreator_MainCheckoutHasNoBase_issue21(t *testing.T) {
	got, err := registry.NewCreator(&FakeGit{Branch: "main"}, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"}, stubID).
		Create(baseState(), "omatty", "poke", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Base != "" {
		t.Errorf("Base = %q, want empty: a main-checkout session diffs against HEAD", got.Base)
	}
}

func TestCreator_UnknownProjectNamesItAndTheKnownOnes(t *testing.T) {
	_, err := registry.NewCreator(&FakeGit{}, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"}, stubID).
		Create(baseState(), "ghost", "t", "")

	if err == nil {
		t.Fatal("Create() with an unknown project returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error %q does not name the offending project %q", err, "ghost")
	}
	if !strings.Contains(err.Error(), "omatty") {
		t.Errorf("error %q does not list the known projects", err)
	}
}

func TestCreator_WorktreeFailureAddsNoSession(t *testing.T) {
	g := &FakeGit{AddErr: errors.New("branch exists")}
	st := baseState()

	if _, err := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: "/home/u/.omatty/wt"}, stubID).
		Create(st, "omatty", "t", "dup"); err == nil {
		t.Fatal("Create() returned nil after a worktree failure, want an error")
	}
	if len(st.Sessions) != 0 {
		t.Errorf("state holds %d sessions after a failure, want 0", len(st.Sessions))
	}
}

func TestCreator_ConfiguredBaseBranchOverridesTheCurrentBranch_issue44(t *testing.T) {
	g := &FakeGit{Branch: "main"}
	c := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: "/vol/wt", BaseBranch: "develop"}, stubID)

	got, err := c.Create(baseState(), "omatty", "parser", "parser-fix")

	if err != nil {
		t.Fatal(err)
	}
	if g.AddedFrom != "develop" || got.Base != "develop" {
		t.Errorf("worktree forked from %q, session records %q; want develop for both", g.AddedFrom, got.Base)
	}
	if g.CurrentBranchCalls != 0 {
		t.Errorf("CurrentBranch asked %d times with a configured base, want 0", g.CurrentBranchCalls)
	}
}

func TestCreator_NoConfiguredBaseFallsBackToTheCurrentBranch_issue44(t *testing.T) {
	g := &FakeGit{Branch: "main"}
	c := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: "/vol/wt"}, stubID)
	got, err := c.Create(baseState(), "omatty", "parser", "parser-fix")
	if err != nil || got.Base != "main" {
		t.Fatalf("Base = %q err = %v, want main from the checkout", got.Base, err)
	}
}

func TestCreator_PlacesTheWorktreeUnderTheConfiguredRoot_issue44(t *testing.T) {
	g := &FakeGit{Branch: "main"}
	c := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: "/vol/wt"}, stubID)
	got, err := c.Create(baseState(), "omatty", "parser", "parser-fix")
	if err != nil || got.Dir != "/vol/wt/omatty/parser-fix" {
		t.Fatalf("Dir = %q err = %v, want /vol/wt/omatty/parser-fix", got.Dir, err)
	}
}

// A session created before the work it would describe exists is registered
// under a placeholder; its first prompt names it (#127).
func TestCreator_ABlankTitleBecomesThePlaceholder_issue127(t *testing.T) {
	c := registry.NewCreator(&FakeGit{Branch: "main"}, registry.CreatorOpts{WorktreeRoot: "/vol/wt"}, stubID)
	sess, err := c.Create(baseState(), "omatty", "  ", "")
	if err != nil || sess.Title != registry.PlaceholderTitle("fixed-uuid") {
		t.Fatalf("Title = %q err = %v, want the placeholder %q", sess.Title, err, registry.PlaceholderTitle("fixed-uuid"))
	}
}

func TestCreator_ATypedTitleIsKept_issue127(t *testing.T) {
	c := registry.NewCreator(&FakeGit{Branch: "main"}, registry.CreatorOpts{WorktreeRoot: "/vol/wt"}, stubID)
	sess, err := c.Create(baseState(), "omatty", "parser fix", "")
	if err != nil || sess.Title != "parser fix" {
		t.Fatalf("Title = %q err = %v, want parser fix", sess.Title, err)
	}
}
