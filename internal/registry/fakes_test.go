package registry_test

import "fmt"

// FakeGit records worktree calls and returns canned results. A named type,
// per AGENTS.md, so a failure message says what stood in for git.
type FakeGit struct {
	Branch    string
	AddErr    error
	AddedDir  string
	AddedBase string
	Removed   []string
}

// RepoRoot echoes dir, so tests can pick the project name by choosing a path.
func (f *FakeGit) RepoRoot(dir string) (string, error) { return dir, nil }

func (f *FakeGit) CurrentBranch(string) (string, error) { return f.Branch, nil }

func (f *FakeGit) RemoveWorktree(_, dir string) error {
	f.Removed = append(f.Removed, dir)
	return nil
}

func (f *FakeGit) AddWorktree(repoRoot, dir, branch string) error {
	if f.AddErr != nil {
		return fmt.Errorf("FakeGit: refusing to add worktree %q on %q: %w", dir, branch, f.AddErr)
	}
	f.AddedBase, f.AddedDir = repoRoot, dir
	return nil
}
