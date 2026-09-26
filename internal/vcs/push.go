package vcs

import (
	"fmt"
	"strings"
)

// remoteName is the remote omatty pushes to. One name, not a guess: `origin` is
// what `git clone` and `gh repo clone` both create, and a checkout that calls
// its remote something else is a checkout omatty declines to ship from rather
// than one it picks a remote for on the operator's behalf (#331).
const remoteName = "origin"

// Push sends branch to origin and sets its upstream (#331).
//
//	err := vcs.NewCLI().Push(sess.Dir, sess.Branch)
//
// **Never forced.** A branch the remote has moved on is a refusal carrying git's
// own "rejected" message, not an overwrite - ROADMAP's shipping section says so
// twice, and it is the one thing a ship key must not be able to do. There is a
// test that diverges a branch and asserts the remote's commit survives.
func (c *CLI) Push(dir, branch string) error {
	if branch == "" {
		return fmt.Errorf("vcs: pushing from %q: no branch to push", dir)
	}
	// --set-upstream so the next `git status` in that worktree says how it
	// stands against the remote, which is what the operator will look at.
	if _, err := c.capture(dir, 0, "push", "--set-upstream", remoteName, branch); err != nil {
		return fmt.Errorf("vcs: pushing %q from %q: %w", branch, dir, err)
	}
	return nil
}

// HasRemote reports whether dir has an origin to push to.
//
//	ok, err := vcs.NewCLI().HasRemote(sess.Dir)
//
// Asked before pushing so "there is nowhere to send this" is a sentence the
// operator reads, rather than git's own error about a repository that does not
// exist - which reads like a fault in omatty.
func (c *CLI) HasRemote(dir string) (bool, error) {
	out, err := c.run(dir, "remote")
	if err != nil {
		return false, err
	}
	for _, name := range strings.Fields(out) {
		if name == remoteName {
			return true, nil
		}
	}
	return false, nil
}
