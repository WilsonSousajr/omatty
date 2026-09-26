package vcs_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// bareRemote gives dir an `origin` pointing at a bare repository, so a push
// can be tested for real without a forge.
func bareRemote(t *testing.T, dir string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitOut(t, dir, "init", "--bare", remote)
	gitOut(t, dir, "remote", "add", "origin", remote)
	return remote
}

// #331 step 1 is push-then-open-a-pull-request, and omatty had no push at all:
// nothing in the repository knew a remote existed.
func TestPush_SendsTheBranchAndSetsItsUpstream_issue331(t *testing.T) {
	dir := newRepo(t)
	remote := bareRemote(t, dir)
	write(t, filepath.Join(dir, "a.go"), "package a\n")
	gitOut(t, dir, "add", "a.go")
	gitOut(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "one")

	if err := vcs.NewCLI().Push(dir, "main"); err != nil {
		t.Fatal(err)
	}

	if got := gitOut(t, remote, "rev-parse", "main"); got != gitOut(t, dir, "rev-parse", "main") {
		t.Errorf("the remote is at %q, want the local commit", got)
	}
	if got := gitOut(t, dir, "rev-parse", "--abbrev-ref", "main@{upstream}"); got != "origin/main" {
		t.Errorf("upstream = %q, want origin/main", got)
	}
}

// Never a force-push: a branch the remote has moved on is a refusal, not an
// overwrite. #331 says so and ROADMAP's shipping section says so twice.
func TestPush_RefusesRatherThanForcing_issue331(t *testing.T) {
	dir := newRepo(t)
	remote := bareRemote(t, dir)
	write(t, filepath.Join(dir, "a.go"), "package a\n")
	gitOut(t, dir, "add", "a.go")
	gitOut(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "one")
	if err := vcs.NewCLI().Push(dir, "main"); err != nil {
		t.Fatal(err)
	}
	// The remote gains a commit the local branch does not have.
	other := filepath.Join(t.TempDir(), "clone")
	gitOut(t, dir, "clone", remote, other)
	write(t, filepath.Join(other, "b.go"), "package b\n")
	gitOut(t, other, "add", "b.go")
	gitOut(t, other, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "two")
	gitOut(t, other, "push", "origin", "main")
	// And the local branch diverges.
	write(t, filepath.Join(dir, "c.go"), "package c\n")
	gitOut(t, dir, "add", "c.go")
	gitOut(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "three")

	err := vcs.NewCLI().Push(dir, "main")

	if err == nil {
		t.Fatal("a diverged branch was pushed anyway; a force-push is never omatty's to make")
	}
	if got := gitOut(t, remote, "log", "--oneline", "-1", "main"); !strings.Contains(got, "two") {
		t.Errorf("the remote's own commit was overwritten: %q", got)
	}
}

// A checkout with no remote is a refusal with a reason, not a git error the
// operator has to read backwards.
func TestHasRemote_TellsAnOriginlessCheckoutApart_issue331(t *testing.T) {
	dir := newRepo(t)

	has, err := vcs.NewCLI().HasRemote(dir)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("HasRemote = true with no remote configured")
	}

	bareRemote(t, dir)

	if has, err = vcs.NewCLI().HasRemote(dir); err != nil || !has {
		t.Errorf("HasRemote = %v, %v; want true once origin exists", has, err)
	}
}
