// Package vcs is omatty's only route to git.
//
// Invariant 4: no other package shells out to git or imports a git library.
// Worktrees go through the git CLI rather than go-git, whose linked-worktree
// support is v6-experimental and implements only add and remove.
package vcs

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// Git is the surface omatty uses. Fake it in tests; do not fake exec.Cmd.
type Git interface {
	RepoRoot(dir string) (string, error)
	CurrentBranch(dir string) (string, error)
	AddWorktree(repoRoot, dir, branch, base string) error
	RemoveWorktree(repoRoot, dir string) error
	// MergeBase returns the commit where ref and dir's HEAD diverged.
	MergeBase(dir, ref string) (string, error)
	// Diff returns the unified diff of dir's working tree against commit, so
	// committed and uncommitted changes appear as one diff (#21).
	Diff(dir, commit string) (string, error)
	// Untracked lists files git does not track, honouring .gitignore.
	Untracked(dir string) ([]string, error)
	// UntrackedDiff renders one untracked file as an all-additions diff.
	UntrackedDiff(dir, path string) (string, error)
	// ListFiles lists tracked and untracked files under dir, honouring
	// .gitignore, sorted (#24).
	ListFiles(dir string) ([]string, error)
}

// CLI runs the real git binary.
//
//	branch, err := vcs.NewCLI().CurrentBranch("/p/omatty")
type CLI struct{ bin string }

// NewCLI returns a CLI that invokes "git" from PATH.
func NewCLI() *CLI { return &CLI{bin: "git"} }

// capture executes git in dir and returns stdout untouched: diff output needs
// its final newline. okExit is one extra exit status treated as success,
// because `git diff --no-index` exits 1 to mean "differences found", which is
// the answer rather than a failure (#21).
func (c *CLI) capture(dir string, okExit int, args ...string) (string, error) {
	if err := checkDir(dir); err != nil {
		return "", err
	}
	cmd := exec.Command(c.bin, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && !exitedWith(err, okExit) {
		return "", fmt.Errorf("vcs: `git %s` in %q failed: %s: %w",
			strings.Join(args, " "), dir, strings.TrimSpace(stderr.String()), err)
	}
	return string(out), nil
}

// exitedWith reports whether err is git exiting with exactly code. A zero
// code never matches: success is not an error in the first place.
func exitedWith(err error, code int) bool {
	var exit *exec.ExitError
	return code != 0 && errors.As(err, &exit) && exit.ExitCode() == code
}

// run executes git in dir and returns trimmed stdout. Failures carry git's
// own stderr, which is the only useful diagnostic a caller can act on.
func (c *CLI) run(dir string, args ...string) (string, error) {
	out, err := c.capture(dir, 0, args...)
	return strings.TrimSpace(out), err
}

// checkDir rejects a bad path before exec. exec.Cmd only fails when it tries
// to chdir, so without this the caller is told "fork/exec /opt/homebrew/bin/
// git: not a directory" - which blames the git binary for the caller's path
// (issue #29).
func checkDir(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("vcs: %q does not exist", dir)
	}
	if err != nil {
		return fmt.Errorf("vcs: cannot read %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vcs: %q is not a directory", dir)
	}
	return nil
}

// RepoRoot returns the top level of the working tree containing dir.
func (c *CLI) RepoRoot(dir string) (string, error) {
	return c.run(dir, "rev-parse", "--show-toplevel")
}

// CurrentBranch returns the branch checked out in dir.
func (c *CLI) CurrentBranch(dir string) (string, error) {
	return c.run(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// AddWorktree creates a linked worktree at dir on a new branch forked from
// base, named explicitly so the recorded base and the fork point agree (#21).
func (c *CLI) AddWorktree(repoRoot, dir, branch, base string) error {
	_, err := c.run(repoRoot, "worktree", "add", "-b", branch, dir, base)
	return err
}

// RemoveWorktree deletes a linked worktree, discarding uncommitted changes.
func (c *CLI) RemoveWorktree(repoRoot, dir string) error {
	_, err := c.run(repoRoot, "worktree", "remove", "--force", dir)
	return err
}

// diffArgs keeps diff output machine-readable: no colour, no external diff
// tool, no path quoting, renames detected so a moved file is one entry.
func diffArgs(extra ...string) []string {
	return append([]string{"-c", "core.quotepath=false", "diff",
		"--no-color", "--no-ext-diff", "-M"}, extra...)
}

// MergeBase returns the commit where ref and HEAD diverged.
//
//	base, err := vcs.NewCLI().MergeBase("/wt/parser-fix", "develop")
func (c *CLI) MergeBase(dir, ref string) (string, error) {
	return c.run(dir, "merge-base", ref, "HEAD")
}

// Diff returns the working tree's unified diff against commit, which is
// everything a session changed whether it committed it or not (#21).
//
//	raw, err := vcs.NewCLI().Diff("/wt/parser-fix", base)
func (c *CLI) Diff(dir, commit string) (string, error) {
	return c.capture(dir, 0, diffArgs(commit, "--")...)
}

// Untracked lists untracked, non-ignored files relative to dir.
//
//	files, err := vcs.NewCLI().Untracked("/wt/parser-fix")
func (c *CLI) Untracked(dir string) ([]string, error) {
	out, err := c.run(dir, "ls-files", "--others", "--exclude-standard")
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

// UntrackedDiff diffs path against /dev/null so a new file reads as pure
// additions; git exits 1 for "differences", which capture tolerates.
//
//	raw, err := vcs.NewCLI().UntrackedDiff("/wt/parser-fix", "new.txt")
func (c *CLI) UntrackedDiff(dir, path string) (string, error) {
	return c.capture(dir, 1, diffArgs("--no-index", "--", os.DevNull, path)...)
}

// ListFiles returns tracked plus untracked, non-ignored paths, sorted. git
// emits the two sets one after the other, so the sort is what makes the tree
// read as a directory listing rather than as two interleaved lists (#24).
//
//	files, err := vcs.NewCLI().ListFiles(sess.Dir)
func (c *CLI) ListFiles(dir string) ([]string, error) {
	out, err := c.run(dir, "ls-files", "--cached", "--others", "--exclude-standard")
	if err != nil || out == "" {
		return nil, err
	}
	files := strings.Split(out, "\n")
	sort.Strings(files)
	return files, nil
}
