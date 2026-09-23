package registry_test

import (
	"fmt"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// FakeGit records worktree calls and returns canned results. A named type,
// per AGENTS.md, so a failure message says what stood in for git.
type FakeGit struct {
	// RenamedFrom and RenamedTo record a branch rename (#151), Commits is
	// what CommitsOnBranch reports.
	RenamedFrom, RenamedTo string
	RenameErr              error
	Commits                int
	CommitsErr             error
	Branch                 string
	AddErr                 error
	AddedDir               string
	AddedRoot              string
	// AddedFrom is the start point the worktree was forked from (#21).
	AddedFrom string
	Removed   []string
	// CurrentBranchCalls counts the git forks a configured base saves (#44).
	CurrentBranchCalls int
}

// RepoRoot echoes dir, so tests can pick the project name by choosing a path.
func (f *FakeGit) RepoRoot(dir string) (string, error) { return dir, nil }

// MainCheckout echoes dir too: registry never calls it, and discovery (#91)
// fakes the whole interface for itself.
func (f *FakeGit) MainCheckout(dir string) (string, error) { return dir, nil }

func (f *FakeGit) CurrentBranch(string) (string, error) {
	f.CurrentBranchCalls++
	return f.Branch, nil
}

func (f *FakeGit) RemoveWorktree(_, dir string) error {
	f.Removed = append(f.Removed, dir)
	return nil
}

// The diff surface exists for review (#21); registry never calls it, so the
// fake answers empty rather than pretending to have a repository.
func (f *FakeGit) MergeBase(_, ref string) (string, error) { return ref, nil }

func (f *FakeGit) Diff(string, string) (string, error) { return "", nil }

func (f *FakeGit) Shortstat(string, string) (vcs.Shortstat, error) { return vcs.Shortstat{}, nil }

func (f *FakeGit) Untracked(string) ([]string, error) { return nil, nil }

func (f *FakeGit) UntrackedDiff(string, string) (string, error) { return "", nil }

func (f *FakeGit) ListFiles(string) ([]string, error) { return nil, nil }

func (f *FakeGit) AddWorktree(repoRoot, dir, branch, base string) error {
	if f.AddErr != nil {
		return fmt.Errorf("FakeGit: refusing to add worktree %q on %q: %w", dir, branch, f.AddErr)
	}
	f.AddedRoot, f.AddedDir, f.AddedFrom = repoRoot, dir, base
	return nil
}

// RenameBranch records the rename #151 asks for; git's own behaviour is
// proved against the real binary in internal/vcs.
func (f *FakeGit) RenameBranch(_, old, name string) error {
	f.RenamedFrom, f.RenamedTo = old, name
	return f.RenameErr
}

// CommitsOnBranch reports what the test set: zero is a branch nobody has
// committed to, which is the only one #151 renames.
func (f *FakeGit) CommitsOnBranch(string, string, string) (int, error) {
	return f.Commits, f.CommitsErr
}
