package vcs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// bareRemote gives dir an `origin` pointing at a bare repository, so a push
// can be tested for real without a forge.
//
// The bare repository's own HEAD follows init.defaultBranch, which differs
// between machines, so nothing here may assume a clone of it lands on `main`.
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
	gitOut(t, dir, "clone", "--branch", "main", remote, other)
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

// Regression, issue #472: git prompts for credentials on the *controlling
// terminal*, which is the terminal omatty is drawing on. In a real PTY the
// bottom of the screen became `sername for 'https://github.com':` - git's own
// half-scrolled prompt over omatty's frame. Invariant 5 says stdout belongs to
// the TUI; a subprocess that prompts takes it anyway.
//
// With prompts disabled git fails immediately instead, with a sentence Push
// already wraps and the footer already shows.
func TestPush_DisablesGitsTerminalPrompt_issue472(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "env")
	bin := filepath.Join(dir, "git")
	script := "#!/bin/sh\nprintenv > '" + log + "'\nexit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := vcs.NewCLIWithBin(bin).Push(dir, "main"); err != nil {
		t.Fatal(err)
	}

	env, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the fake git recorded no environment: %v", err)
	}
	if !strings.Contains(string(env), "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("the push does not disable git's terminal prompt, so a remote "+
			"needing credentials writes over the TUI; env was:\n%s", env)
	}
}
