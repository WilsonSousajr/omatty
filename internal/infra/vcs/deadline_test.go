package vcs_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/infra/vcs"
)

// hungGit is a stand-in git that never answers: what a network filesystem, a
// lock or a credential prompt looks like from omatty's side.
func hungGit(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "git")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 60\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin
}

// Regression, issue #650: git ran with no deadline, and several calls run on
// the TUI's event loop (git worktree add on ctrl+o N, rev-parse per root on
// discovery), so a git that hung froze omatty until it was killed. Now it is
// cut off and reported, naming the deadline, instead.
func TestCLI_aHungGitIsCutOffAtItsDeadline_issue650(t *testing.T) {
	cli := vcs.NewCLIWithDeadline(hungGit(t), 300*time.Millisecond)
	start := time.Now()

	_, err := cli.CurrentBranch(t.TempDir())

	if err == nil {
		t.Fatal("a hung git returned no error")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("a 300 ms deadline returned after %v: the kill did not stop the wait", elapsed)
	}
	var cmdErr *vcs.CommandError
	if !errors.As(err, &cmdErr) || !strings.Contains(err.Error(), "did not finish within 300ms") {
		t.Errorf("error = %v, want a CommandError saying git did not finish within 300ms", err)
	}
}

// check-attr --stdin has its own exec site; it is held to the same deadline.
func TestCLI_aHungCheckAttrIsCutOff_issue650(t *testing.T) {
	cli := vcs.NewCLIWithDeadline(hungGit(t), 300*time.Millisecond)

	_, err := cli.Attr(t.TempDir(), "linguist-generated", []string{"a.go"})

	if err == nil || !strings.Contains(err.Error(), "did not finish within") {
		t.Errorf("Attr on a hung git = %v, want it cut off at its deadline", err)
	}
}

// Every git call has 30 s; git worktree add, which checks a tree out, has 60.
func TestCLI_deadlinesAreNamedPerCommand_issue650(t *testing.T) {
	if got := vcs.DeadlineFor("worktree", "add", "-b", "x", "/d"); got != 60*time.Second {
		t.Errorf("worktree add deadline = %v, want 60s", got)
	}
	for _, args := range [][]string{{"rev-parse", "--show-toplevel"}, {"worktree", "remove", "/d"}, {"diff"}} {
		if got := vcs.DeadlineFor(args...); got != 30*time.Second {
			t.Errorf("%v deadline = %v, want 30s", args, got)
		}
	}
}
