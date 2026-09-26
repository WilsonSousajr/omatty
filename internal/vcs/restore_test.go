package vcs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// #334's whole mechanism: the turn baseline is a tree object (#311), so a
// revert is that tree written back over the working directory.
func TestRestoreTree_PutsTheWorkingTreeBackAsItWas_issue334(t *testing.T) {
	dir := newRepo(t)
	git := vcs.NewCLI()
	writeIn(t, dir, "kept.go", "package a\n\nfunc F() {}\n")
	writeIn(t, dir, "doomed.go", "package a\n")
	before, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	// The turn: one file edited, one deleted, one created.
	writeIn(t, dir, "kept.go", "package a\n\nfunc F() { panic(1) }\n")
	if err := os.Remove(filepath.Join(dir, "doomed.go")); err != nil {
		t.Fatal(err)
	}
	writeIn(t, dir, "invented.go", "package a\n")

	if err := git.RestoreTree(dir, before); err != nil {
		t.Fatal(err)
	}

	if got := read(t, dir, "kept.go"); got != "package a\n\nfunc F() {}\n" {
		t.Errorf("kept.go = %q, want the edit undone", got)
	}
	if got := read(t, dir, "doomed.go"); got != "package a\n" {
		t.Errorf("doomed.go = %q, want the deletion undone", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "invented.go")); !os.IsNotExist(err) {
		t.Error("invented.go survived; a file the turn created is part of the turn")
	}
}

// The requirement that makes it safe to press: a gitignored file that existed
// before the turn is not the turn's, and must not be touched. SnapshotTree
// honours .gitignore, so such a file is in neither tree - and the removal pass
// has to honour it too, or a revert deletes the .env the session needed.
func TestRestoreTree_LeavesAGitignoredFileAlone_issue334(t *testing.T) {
	dir := newRepo(t)
	git := vcs.NewCLI()
	writeIn(t, dir, ".gitignore", ".env\n")
	writeIn(t, dir, ".env", "TOKEN=before\n")
	writeIn(t, dir, "a.go", "package a\n")
	before, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeIn(t, dir, "a.go", "package b\n")

	if err := git.RestoreTree(dir, before); err != nil {
		t.Fatal(err)
	}

	if got := read(t, dir, ".env"); got != "TOKEN=before\n" {
		t.Errorf(".env = %q, want it untouched by the revert", got)
	}
}

// The discipline SnapshotTree set (#311): the operator's own index, HEAD and
// stash are not the mechanism's to move. A revert restores the *working tree*.
func TestRestoreTree_TouchesNeitherTheIndexNorHead_issue334(t *testing.T) {
	dir := newRepo(t)
	git := vcs.NewCLI()
	writeIn(t, dir, "a.go", "package a\n")
	gitOut(t, dir, "add", "a.go")
	gitOut(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "one")
	writeIn(t, dir, "staged.go", "package staged\n")
	gitOut(t, dir, "add", "staged.go")
	head := gitOut(t, dir, "rev-parse", "HEAD")
	stagedBefore := gitOut(t, dir, "diff", "--cached", "--name-only")

	before, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeIn(t, dir, "a.go", "package changed\n")
	if err := git.RestoreTree(dir, before); err != nil {
		t.Fatal(err)
	}

	if got := gitOut(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved: %q, want %q", got, head)
	}
	if got := gitOut(t, dir, "diff", "--cached", "--name-only"); got != stagedBefore {
		t.Errorf("the index changed: %q, want %q", got, stagedBefore)
	}
}

// A tree that does not exist is an error naming it, not a working tree left
// half written.
func TestRestoreTree_ReportsATreeThatIsNotThere_issue334(t *testing.T) {
	if err := vcs.NewCLI().RestoreTree(newRepo(t), "0000000000000000000000000000000000000000"); err == nil {
		t.Error("restoring a tree that does not exist should fail")
	}
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
