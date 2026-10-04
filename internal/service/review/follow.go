package review

import (
	"fmt"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
)

// Follow is sess as the review should read it once its agent reports working
// in cwd (#659): Dir moved to the top level of the checkout cwd is in, and
// Branch to that checkout's branch - none for the main checkout, which is
// diffed against HEAD as a main-checkout session is.
//
// claude enters worktrees and cds into subdirectories during a session, and
// the review kept reading the directory omatty launched it in. A subdirectory
// of the session's own checkout changes nothing, and neither does a checkout
// of another repository: the gate and ship would then act on code the session
// does not own. sess itself is never changed, so the transcript path,
// --resume and the archive keep the launch directory (invariant 9).
//
//	followed, err := src.Follow(sess, "/repo/.worktrees/fix/internal/ui")
func (s *Source) Follow(sess session.Session, cwd string) (session.Session, error) {
	top, err := s.git.RepoRoot(cwd)
	if err != nil {
		return sess, fmt.Errorf("review: session %s works in %q, which is not inside a git checkout: %w", sess.ID, cwd, err)
	}
	home, err := s.git.RepoRoot(sess.Dir)
	if err != nil || top == home {
		return sess, err
	}
	same, main, err := s.sameRepository(sess.Dir, top)
	if err != nil || !same {
		return sess, err
	}
	return s.readAt(sess, top, main)
}

// sameRepository reports whether two checkouts share one main checkout, and
// names it.
func (s *Source) sameRepository(home, top string) (bool, string, error) {
	homeMain, err := s.git.MainCheckout(home)
	if err != nil {
		return false, "", fmt.Errorf("review: main checkout of %q: %w", home, err)
	}
	main, err := s.git.MainCheckout(top)
	if err != nil {
		return false, "", fmt.Errorf("review: main checkout of %q: %w", top, err)
	}
	return main == homeMain, main, nil
}

// readAt is sess pointed at the checkout top, on its branch.
func (s *Source) readAt(sess session.Session, top, main string) (session.Session, error) {
	sess.Dir, sess.Branch = top, ""
	if top == main {
		return sess, nil
	}
	branch, err := s.git.CurrentBranch(top)
	if err != nil {
		return sess, fmt.Errorf("review: branch of the checkout %q: %w", top, err)
	}
	sess.Branch = branch
	return sess, nil
}
