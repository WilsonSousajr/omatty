package review_test

import (
	"fmt"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// FakeGit answers the diff surface from canned values and records the calls in
// order, so a test can assert which ref was diffed against. A named type, per
// AGENTS.md, so a failure message says what stood in for git.
type FakeGit struct {
	// RenamedFrom and RenamedTo record a branch rename (#151), Commits is
	// what CommitsOnBranch reports.
	RenamedFrom, RenamedTo string
	RenameErr              error
	Commits                int
	CommitsErr             error
	Branch                 string            // CurrentBranch of any dir
	MergeBaseOut           string            // MergeBase result
	DiffOut                string            // Diff result
	ShortstatOut           vcs.Shortstat     // Shortstat result (#180)
	UntrackedOut           []string          // Untracked result
	FileDiffs              map[string]string // UntrackedDiff result per path
	Files                  []string          // ListFiles result (#24)
	SnapshotOut            string            // SnapshotTree result (#311)
	TurnTree               string            // TurnRef's tree; empty means no baseline
	DiffTreesOut           string            // DiffTrees result
	HeadOut                string            // Head result (#310)
	AttrOut                map[string]bool   // Attr result (#338)
	Err                    error             // returned by every method when set
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

func (f *FakeGit) Shortstat(dir, commit string) (vcs.Shortstat, error) {
	return f.ShortstatOut, f.record("Shortstat", dir, commit)
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

func (f *FakeGit) SnapshotTree(dir string) (string, error) {
	return f.SnapshotOut, f.record("SnapshotTree", dir)
}

func (f *FakeGit) SetTurnRef(dir, id, tree string) error {
	return f.record("SetTurnRef", dir, id, tree)
}

func (f *FakeGit) TurnRef(dir, id string) (string, bool, error) {
	return f.TurnTree, f.TurnTree != "", f.record("TurnRef", dir, id)
}

func (f *FakeGit) DeleteTurnRef(dir, id string) error {
	return f.record("DeleteTurnRef", dir, id)
}

func (f *FakeGit) DiffTrees(dir, from, to string) (string, error) {
	return f.DiffTreesOut, f.record("DiffTrees", dir, from, to)
}

func (f *FakeGit) Head(dir string) (string, error) {
	return f.HeadOut, f.record("Head", dir)
}

func (f *FakeGit) Attr(dir, attr string, paths []string) (map[string]bool, error) {
	return f.AttrOut, f.record("Attr", dir, attr, strings.Join(paths, " "))
}
