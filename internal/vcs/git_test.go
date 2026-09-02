package vcs_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// newRepo builds a real one-commit git repository in a temp dir. omatty
// depends on the git binary anyway, so exercising it is the honest test.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"commit", "--allow-empty", "-m", "root"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func TestCLI_CurrentBranch(t *testing.T) {
	got, err := vcs.NewCLI().CurrentBranch(newRepo(t))
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v, want nil", err)
	}
	if got != "main" {
		t.Errorf("CurrentBranch() = %q, want %q", got, "main")
	}
}

func TestCLI_RepoRootFromASubdirectory(t *testing.T) {
	repo := newRepo(t)
	sub := filepath.Join(repo, "internal", "ui")
	if err := exec.Command("mkdir", "-p", sub).Run(); err != nil {
		t.Fatal(err)
	}

	got, err := vcs.NewCLI().RepoRoot(sub)

	if err != nil {
		t.Fatalf("RepoRoot() error = %v, want nil", err)
	}
	// macOS reports /private/var for /var, so compare resolved names.
	if filepath.Base(got) != filepath.Base(repo) {
		t.Errorf("RepoRoot(%q) = %q, want the repo root %q", sub, got, repo)
	}
}

func TestCLI_RepoRootOutsideARepoNamesTheDirectory(t *testing.T) {
	dir := t.TempDir()

	_, err := vcs.NewCLI().RepoRoot(dir)

	if err == nil {
		t.Fatal("RepoRoot() outside a repository returned nil, want an error")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("error %q does not name the offending directory %q", err, dir)
	}
}

func TestCLI_AddAndRemoveWorktree(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(t.TempDir(), "parser-fix")
	g := vcs.NewCLI()

	if err := g.AddWorktree(repo, wt, "parser-fix"); err != nil {
		t.Fatalf("AddWorktree() error = %v, want nil", err)
	}
	branch, err := g.CurrentBranch(wt)
	if err != nil {
		t.Fatalf("CurrentBranch(worktree) error = %v, want nil", err)
	}
	if branch != "parser-fix" {
		t.Errorf("worktree is on %q, want %q", branch, "parser-fix")
	}
	if err := g.RemoveWorktree(repo, wt); err != nil {
		t.Errorf("RemoveWorktree() error = %v, want nil", err)
	}
}

func TestCLI_AddWorktreeOnTheCheckedOutBranchReportsStderr(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(t.TempDir(), "dup")

	err := vcs.NewCLI().AddWorktree(repo, wt, "main")

	if err == nil {
		t.Fatal("AddWorktree() on the checked-out branch returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "main") {
		t.Errorf("error %q does not name the offending branch %q", err, "main")
	}
}

func TestCLI_RemoveMissingWorktreeNamesThePath(t *testing.T) {
	repo := newRepo(t)
	missing := filepath.Join(t.TempDir(), "never-created")

	err := vcs.NewCLI().RemoveWorktree(repo, missing)

	if err == nil {
		t.Fatal("RemoveWorktree() on a missing worktree returned nil, want an error")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not name the offending path %q", err, missing)
	}
}
