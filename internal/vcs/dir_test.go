package vcs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// Regression, issue #29: exec.Cmd fails at exec time when Dir is not a
// directory, so the error blamed the git binary ("fork/exec /opt/homebrew/bin/
// git: not a directory") for the caller's bad path.
func TestCLI_PathIsAFileNamesItAsAFile_issue29(t *testing.T) {
	file := filepath.Join(t.TempDir(), "omatty")
	if err := os.WriteFile(file, []byte("a binary, not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := vcs.NewCLI().RepoRoot(file)

	if err == nil {
		t.Fatal("RepoRoot() on a file returned nil, want an error")
	}
	if !strings.Contains(err.Error(), file) {
		t.Errorf("error %q does not name the offending path", err)
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error %q does not say the path is not a directory", err)
	}
	if strings.Contains(err.Error(), "fork/exec") || strings.Contains(err.Error(), "git rev-parse") {
		t.Errorf("error %q leaks exec internals and blames git for the caller's path", err)
	}
}

func TestCLI_PathDoesNotExistSaysSo_issue29(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")

	_, err := vcs.NewCLI().RepoRoot(missing)

	if err == nil {
		t.Fatal("RepoRoot() on a missing path returned nil, want an error")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not name the offending path", err)
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q does not say the path does not exist", err)
	}
	if strings.Contains(err.Error(), "chdir") {
		t.Errorf("error %q leaks the chdir syscall", err)
	}
}

// Every git command shares the guard, not just the one `add` happens to use.
func TestCLI_AllCommandsValidateTheDirectory_issue29(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	g := vcs.NewCLI()

	checks := map[string]error{
		"CurrentBranch":  func() error { _, err := g.CurrentBranch(missing); return err }(),
		"AddWorktree":    g.AddWorktree(missing, "/tmp/wt", "b", "main"),
		"RemoveWorktree": g.RemoveWorktree(missing, "/tmp/wt"),
		"MergeBase":      func() error { _, err := g.MergeBase(missing, "main"); return err }(),
		"Diff":           func() error { _, err := g.Diff(missing, "HEAD"); return err }(),
		"Untracked":      func() error { _, err := g.Untracked(missing); return err }(),
		"UntrackedDiff":  func() error { _, err := g.UntrackedDiff(missing, "x"); return err }(),
		"ListFiles":      func() error { _, err := g.ListFiles(missing); return err }(),
	}
	for name, err := range checks {
		if err == nil {
			t.Errorf("%s on a missing directory returned nil, want an error", name)
			continue
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("%s error %q does not say the path does not exist", name, err)
		}
	}
}
