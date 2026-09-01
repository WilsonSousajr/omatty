// Package vcs is omatty's only route to git.
//
// Invariant 4: no other package shells out to git or imports a git library.
// Worktrees go through the git CLI rather than go-git, whose linked-worktree
// support is v6-experimental and implements only add and remove.
package vcs

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Git is the surface omatty uses. Fake it in tests; do not fake exec.Cmd.
type Git interface {
	RepoRoot(dir string) (string, error)
	CurrentBranch(dir string) (string, error)
	AddWorktree(repoRoot, dir, branch string) error
	RemoveWorktree(repoRoot, dir string) error
}

// CLI runs the real git binary.
//
//	branch, err := vcs.NewCLI().CurrentBranch("/p/omatty")
type CLI struct{ bin string }

// NewCLI returns a CLI that invokes "git" from PATH.
func NewCLI() *CLI { return &CLI{bin: "git"} }

// run executes git in dir and returns trimmed stdout. Failures carry git's
// own stderr, which is the only useful diagnostic a caller can act on.
func (c *CLI) run(dir string, args ...string) (string, error) {
	cmd := exec.Command(c.bin, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("vcs: `git %s` in %q failed: %s: %w",
			strings.Join(args, " "), dir, strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoRoot returns the top level of the working tree containing dir.
func (c *CLI) RepoRoot(dir string) (string, error) {
	return c.run(dir, "rev-parse", "--show-toplevel")
}

// CurrentBranch returns the branch checked out in dir.
func (c *CLI) CurrentBranch(dir string) (string, error) {
	return c.run(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// AddWorktree creates a linked worktree at dir on a new branch.
func (c *CLI) AddWorktree(repoRoot, dir, branch string) error {
	_, err := c.run(repoRoot, "worktree", "add", "-b", branch, dir)
	return err
}

// RemoveWorktree deletes a linked worktree, discarding uncommitted changes.
func (c *CLI) RemoveWorktree(repoRoot, dir string) error {
	_, err := c.run(repoRoot, "worktree", "remove", "--force", dir)
	return err
}
