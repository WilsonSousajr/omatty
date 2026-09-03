package registry_test

import "fmt"

// FakeGit records worktree calls and returns canned results. A named type,
// per AGENTS.md, so a failure message says what stood in for git.
type FakeGit struct {
	Branch    string
	AddErr    error
	AddedDir  string
	AddedRoot string
	// AddedFrom is the start point the worktree was forked from (#21).
	AddedFrom string
	Removed   []string
}

// RepoRoot echoes dir, so tests can pick the project name by choosing a path.
func (f *FakeGit) RepoRoot(dir string) (string, error) { return dir, nil }

func (f *FakeGit) CurrentBranch(string) (string, error) { return f.Branch, nil }

func (f *FakeGit) RemoveWorktree(_, dir string) error {
	f.Removed = append(f.Removed, dir)
	return nil
}

// The diff surface exists for review (#21); registry never calls it, so the
// fake answers empty rather than pretending to have a repository.
func (f *FakeGit) MergeBase(_, ref string) (string, error) { return ref, nil }

func (f *FakeGit) Diff(string, string) (string, error) { return "", nil }

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
