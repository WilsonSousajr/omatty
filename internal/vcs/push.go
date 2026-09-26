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
	//
	// GIT_TERMINAL_PROMPT=0, because git asks for credentials on the
	// *controlling terminal* - which is the terminal omatty is drawing on. In a
	// real PTY the bottom of the screen became git's own half-scrolled
	// `sername for 'https://github.com':` over omatty's frame (#472).
	// Invariant 5 says stdout belongs to the TUI; a subprocess that prompts
	// takes it anyway, so the only fix is not to let it ask. git then fails at
	// once with "could not read Username ... terminal prompts disabled", which
	// the wrap below turns into a sentence in the footer.
	noPrompt := []string{"GIT_TERMINAL_PROMPT=0"}
	if _, err := c.captureEnv(dir, 0, noPrompt, "push", "--set-upstream", remoteName, branch); err != nil {
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
