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

	got, err := registry.NewCreator(g, "/home/u", stubID).Create(st, "omatty", "poke", "")

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

	got, err := registry.NewCreator(g, "/home/u", stubID).
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
	if g.AddedBase != "/p/omatty" || g.AddedDir != want {
		t.Errorf("AddWorktree(%q, %q), want (%q, %q)", g.AddedBase, g.AddedDir, "/p/omatty", want)
	}
}

func TestCreator_UnknownProjectNamesItAndTheKnownOnes(t *testing.T) {
	_, err := registry.NewCreator(&FakeGit{}, "/home/u", stubID).
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

	if _, err := registry.NewCreator(g, "/home/u", stubID).
		Create(st, "omatty", "t", "dup"); err == nil {
		t.Fatal("Create() returned nil after a worktree failure, want an error")
	}
	if len(st.Sessions) != 0 {
		t.Errorf("state holds %d sessions after a failure, want 0", len(st.Sessions))
	}
}
