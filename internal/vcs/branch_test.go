package vcs_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// #151 renames a worktree's placeholder branch once its first prompt has said
// what the work is. The worktree's HEAD has to follow, and the directory has
// to stay where it is: claude is running in it and the transcript path is
// derived from it, so moving it would blind the tailer (#60).
func TestCLI_RenameBranchFollowsTheWorktreeAndLeavesItsDirectory_issue151(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(t.TempDir(), "omatty-a1b2c3d4")
	g := vcs.NewCLI()
	if err := g.AddWorktree(repo, wt, "omatty-a1b2c3d4", "main"); err != nil {
		t.Fatal(err)
	}

	if err := g.RenameBranch(repo, "omatty-a1b2c3d4", "fix-the-wheel-pan"); err != nil {
		t.Fatalf("RenameBranch() error = %v, want nil", err)
	}

	branch, err := g.CurrentBranch(wt)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "fix-the-wheel-pan" {
		t.Errorf("the worktree is on %q, want the renamed branch", branch)
	}
	if got := gitOut(t, wt, "rev-parse", "--show-toplevel"); got != evalSymlinks(t, wt) {
		t.Errorf("the worktree moved to %q, want it left at %q", got, wt)
	}
}

// A name already taken is refused rather than forced: -m, never -M. The error
// names the branch, so the footer says which one.
func TestCLI_RenameBranchRefusesAnExistingName_issue151(t *testing.T) {
	repo := newRepo(t)
	gitOut(t, repo, "branch", "taken")
	gitOut(t, repo, "branch", "omatty-a1b2c3d4")

	err := vcs.NewCLI().RenameBranch(repo, "omatty-a1b2c3d4", "taken")

	if err == nil {
		t.Fatal("RenameBranch() onto an existing name returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "taken") {
		t.Errorf("error %q does not name the offending branch", err)
	}
}

// The placeholder is renamed only while the branch has nothing of its own on
// it: after the first commit the name is in the history someone may have
// pushed, and renaming it is the operator's call, not omatty's.
func TestCLI_CommitsOnBranchCountsWhatTheBranchAdded_issue151(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(t.TempDir(), "feat")
	g := vcs.NewCLI()
	if err := g.AddWorktree(repo, wt, "feat", "main"); err != nil {
		t.Fatal(err)
	}

	n, err := g.CommitsOnBranch(repo, "main", "feat")
	if err != nil {
		t.Fatalf("CommitsOnBranch() error = %v, want nil", err)
	}
	if n != 0 {
		t.Errorf("a fresh worktree has %d commits of its own, want 0", n)
	}

	gitOut(t, wt, "commit", "--allow-empty", "-m", "work")

	if n, err = g.CommitsOnBranch(repo, "main", "feat"); err != nil || n != 1 {
		t.Errorf("CommitsOnBranch() = %d, %v, want 1, nil", n, err)
	}
}

// evalSymlinks resolves a temp path the way git reports it: macOS hands out
// /var/folders paths that are symlinks to /private/var.
func evalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
