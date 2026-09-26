package review

import (
	"fmt"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// Shippable is what a session must be for #331 to push it: everything the gate
// verified is committed, and there is a commit to send.
type Shippable struct {
	// Uncommitted is how many files differ from HEAD, tracked or not. Zero is
	// the only shippable value.
	Uncommitted int
	// Commits is how many the branch has that its base does not. Zero means
	// there is nothing to open a pull request about.
	Commits int
}

// Shippable reads sess's worktree for #331's two preconditions.
//
//	s, err := src.Shippable(sess, projectRoot)
//
// Read when the ship key is pressed rather than on every card poll: the poll
// already costs two git calls per session per interval, and this answer is only
// ever wanted at the moment somebody asks to ship.
//
// It is deliberately separate from Stat, whose counts are against the *base*
// commit and so include committed work. "Clean" is a different question from
// "what has this session changed", and conflating them is how a pull request
// would come to differ from the tree the gate verified.
func (s *Source) Shippable(sess registry.Session, projectRoot string) (Shippable, error) {
	stat, err := s.git.Shortstat(sess.Dir, "HEAD")
	if err != nil {
		return Shippable{}, fmt.Errorf("review: uncommitted changes of session %s in %q: %w",
			sess.ID, sess.Dir, err)
	}
	untracked, err := s.git.Untracked(sess.Dir)
	if err != nil {
		return Shippable{}, fmt.Errorf("review: untracked files of session %s in %q: %w",
			sess.ID, sess.Dir, err)
	}
	commits, err := s.commitsAhead(sess, projectRoot)
	if err != nil {
		return Shippable{}, err
	}
	return Shippable{Uncommitted: stat.Files + len(untracked), Commits: commits}, nil
}

// commitsAhead is how many commits sess's branch has that its base does not.
//
// A main-checkout session has no branch of its own to count, and no base it
// diverged from; it reports none, which refuses the ship rather than opening a
// pull request from a branch onto itself.
func (s *Source) commitsAhead(sess registry.Session, projectRoot string) (int, error) {
	if sess.Branch == "" || sess.Base == "" {
		return 0, nil
	}
	n, err := s.git.CommitsOnBranch(projectRoot, sess.Base, sess.Branch)
	if err != nil {
		return 0, fmt.Errorf("review: counting %q against %q: %w", sess.Branch, sess.Base, err)
	}
	return n, nil
}
