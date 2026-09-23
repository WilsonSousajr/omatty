package vcs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func treeFiles(t *testing.T, dir, tree string) string {
	t.Helper()
	return gitOut(t, dir, "ls-tree", "-r", "--name-only", tree)
}

// A turn baseline is the working tree as the agent is about to change it:
// the files it created count, the ones .gitignore excludes do not (#311).
func TestCLI_SnapshotTreeTakesUntrackedAndSkipsIgnored_issue311(t *testing.T) {
	dir := newRepo(t)
	write(t, filepath.Join(dir, ".gitignore"), "*.log\n")
	write(t, filepath.Join(dir, "new.txt"), "fresh\n")
	write(t, filepath.Join(dir, "noise.log"), "ignored\n")

	tree, err := vcs.NewCLI().SnapshotTree(dir)
	if err != nil {
		t.Fatalf("SnapshotTree() error = %v", err)
	}

	files := treeFiles(t, dir, tree)
	if !strings.Contains(files, "new.txt") || !strings.Contains(files, ".gitignore") {
		t.Errorf("the snapshot misses an untracked file:\n%s", files)
	}
	if strings.Contains(files, "noise.log") {
		t.Errorf("the snapshot took an ignored file:\n%s", files)
	}
}

// The snapshot goes through a copy of the index. The operator's staging,
// HEAD and stash are theirs, and a baseline that moved any of them would be
// omatty editing the repository behind their back.
func TestCLI_SnapshotTreeLeavesIndexHeadAndStashAlone_issue311(t *testing.T) {
	dir := newRepo(t)
	write(t, filepath.Join(dir, "staged.txt"), "one\n")
	gitOut(t, dir, "add", "staged.txt")
	write(t, filepath.Join(dir, "loose.txt"), "two\n")
	index := filepath.Join(dir, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	head := gitOut(t, dir, "rev-parse", "HEAD")
	status := gitOut(t, dir, "status", "--porcelain")

	if _, err := vcs.NewCLI().SnapshotTree(dir); err != nil {
		t.Fatalf("SnapshotTree() error = %v", err)
	}

	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("the real index changed")
	}
	if got := gitOut(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved from %s to %s", head, got)
	}
	if got := gitOut(t, dir, "status", "--porcelain"); got != status {
		t.Errorf("status changed:\nbefore %q\nafter  %q", status, got)
	}
	if got := gitOut(t, dir, "stash", "list"); strings.TrimSpace(got) != "" {
		t.Errorf("the stash is not empty: %q", got)
	}
}

// A linked worktree has its own index, and refs live in the common
// repository: a worktree session's baseline must read back from anywhere.
func TestCLI_SnapshotTreeInALinkedWorktree_issue311(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	git := vcs.NewCLI()
	if err := git.AddWorktree(repo, wt, "feature", "main"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt, "only-here.txt"), "wt\n")

	tree, err := git.SnapshotTree(wt)
	if err != nil {
		t.Fatalf("SnapshotTree() error = %v", err)
	}
	if err := git.SetTurnRef(wt, "s1", tree); err != nil {
		t.Fatalf("SetTurnRef() error = %v", err)
	}

	got, ok, err := git.TurnRef(repo, "s1")
	if err != nil || !ok || got != tree {
		t.Errorf("TurnRef from the main checkout = %q, %v, %v; want %q, true, nil", got, ok, err, tree)
	}
	if !strings.Contains(treeFiles(t, repo, tree), "only-here.txt") {
		t.Error("the worktree's snapshot does not hold the worktree's file")
	}
}

// A repository nobody has committed to has no index file at all.
func TestCLI_SnapshotTreeInARepoWithNoCommits_issue311(t *testing.T) {
	dir := t.TempDir()
	gitOut(t, dir, "init", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "a\n")

	tree, err := vcs.NewCLI().SnapshotTree(dir)
	if err != nil {
		t.Fatalf("SnapshotTree() error = %v", err)
	}
	if !strings.Contains(treeFiles(t, dir, tree), "a.txt") {
		t.Error("the snapshot of an empty repository misses its file")
	}
}

func TestCLI_DiffTreesReportsAddedModifiedAndDeleted_issue311(t *testing.T) {
	dir := newRepo(t)
	write(t, filepath.Join(dir, "keep.txt"), "old\n")
	write(t, filepath.Join(dir, "gone.txt"), "bye\n")
	git := vcs.NewCLI()
	base, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "keep.txt"), "changed\n")
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "new.txt"), "hello\n")
	now, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := git.DiffTrees(dir, base, now)
	if err != nil {
		t.Fatalf("DiffTrees() error = %v", err)
	}
	for _, want := range []string{"+changed", "deleted file mode", "diff --git a/new.txt b/new.txt", "+hello"} {
		if !strings.Contains(raw, want) {
			t.Errorf("the diff lacks %q:\n%s", want, raw)
		}
	}
}

func TestCLI_TurnRefReadsBackWhatWasSet_issue311(t *testing.T) {
	dir := newRepo(t)
	git := vcs.NewCLI()
	if _, ok, err := git.TurnRef(dir, "s1"); ok || err != nil {
		t.Fatalf("TurnRef before any = ok %v, err %v; want false, nil", ok, err)
	}
	tree, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.SetTurnRef(dir, "s1", tree); err != nil {
		t.Fatalf("SetTurnRef() error = %v", err)
	}
	if got, ok, err := git.TurnRef(dir, "s1"); got != tree || !ok || err != nil {
		t.Errorf("TurnRef = %q, %v, %v; want %q, true, nil", got, ok, err, tree)
	}
}

// Archive deletes the ref and must not fail when there is none to delete.
func TestCLI_DeleteTurnRefRemovesItAndToleratesAMissingOne_issue311(t *testing.T) {
	dir := newRepo(t)
	git := vcs.NewCLI()
	tree, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.SetTurnRef(dir, "s1", tree); err != nil {
		t.Fatal(err)
	}

	if err := git.DeleteTurnRef(dir, "s1"); err != nil {
		t.Fatalf("DeleteTurnRef() error = %v", err)
	}
	if _, ok, _ := git.TurnRef(dir, "s1"); ok {
		t.Error("the ref survived its delete")
	}
	if err := git.DeleteTurnRef(dir, "s1"); err != nil {
		t.Errorf("deleting a missing ref = %v, want nil: archive must not fail on it", err)
	}
}
