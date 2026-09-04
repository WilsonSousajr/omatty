package review_test

import (
	"fmt"
	"strings"
)

// FakeGit answers the diff surface from canned values and records the calls in
// order, so a test can assert which ref was diffed against. A named type, per
// AGENTS.md, so a failure message says what stood in for git.
type FakeGit struct {
	Branch       string            // CurrentBranch of any dir
	MergeBaseOut string            // MergeBase result
	DiffOut      string            // Diff result
	UntrackedOut []string          // Untracked result
	FileDiffs    map[string]string // UntrackedDiff result per path
	Files        []string          // ListFiles result (#24)
	Err          error             // returned by every method when set
	// Errs fails one method by name, so a test can reach an error path that
	// lies behind a call which has to succeed first.
	Errs  map[string]error
	Calls []string
}

func (f *FakeGit) record(name string, args ...string) error {
	f.Calls = append(f.Calls, name+"("+strings.Join(args, ",")+")")
	if err := f.Errs[name]; err != nil {
		return fmt.Errorf("FakeGit %s(%s): %w", name, strings.Join(args, ","), err)
	}
	if f.Err != nil {
		return fmt.Errorf("FakeGit %s: %w", name, f.Err)
	}
	return nil
}

func (f *FakeGit) RepoRoot(dir string) (string, error) { return dir, f.record("RepoRoot", dir) }

func (f *FakeGit) MainCheckout(dir string) (string, error) {
	return dir, f.record("MainCheckout", dir)
}

func (f *FakeGit) CurrentBranch(dir string) (string, error) {
	return f.Branch, f.record("CurrentBranch", dir)
}

func (f *FakeGit) AddWorktree(root, dir, branch, base string) error {
	return f.record("AddWorktree", root, dir, branch, base)
}

func (f *FakeGit) RemoveWorktree(root, dir string) error {
	return f.record("RemoveWorktree", root, dir)
}

func (f *FakeGit) MergeBase(dir, ref string) (string, error) {
	return f.MergeBaseOut, f.record("MergeBase", dir, ref)
}

func (f *FakeGit) Diff(dir, commit string) (string, error) {
	return f.DiffOut, f.record("Diff", dir, commit)
}

func (f *FakeGit) Untracked(dir string) ([]string, error) {
	return f.UntrackedOut, f.record("Untracked", dir)
}

func (f *FakeGit) UntrackedDiff(dir, p string) (string, error) {
	return f.FileDiffs[p], f.record("UntrackedDiff", dir, p)
}

func (f *FakeGit) ListFiles(dir string) ([]string, error) {
	return f.Files, f.record("ListFiles", dir)
}
