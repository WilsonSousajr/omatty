package review

import (
	"errors"
	"fmt"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// ErrNoTurn is LoadTurn's answer for a session with no turn baseline: no
// prompt has been submitted since #311 shipped, or its hooks are not
// arriving (#49). The review column shows it as a notice, not a failure.
var ErrNoTurn = errors.New("review: no turn recorded yet")

// SnapTurn records sess's working tree as the baseline of the turn that is
// starting - what LoadTurn diffs against until the next prompt (#311).
//
//	err := src.SnapTurn(sess)
func (s *Source) SnapTurn(sess registry.Session) error {
	tree, err := s.git.SnapshotTree(sess.Dir)
	if err != nil {
		return fmt.Errorf("review: snapshotting session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	if err := s.git.SetTurnRef(sess.Dir, sess.ID, tree); err != nil {
		return fmt.Errorf("review: recording the turn baseline of session %s: %w", sess.ID, err)
	}
	return nil
}

// LoadTurn is what sess changed since its turn began: the working tree now
// against the baseline SnapTurn recorded. The unused projectRoot makes it a
// DiffFunc like Load, so the UI loads both the same way.
//
//	d, err := src.LoadTurn(sess, projectRoot)
func (s *Source) LoadTurn(sess registry.Session, _ string) (Diff, error) {
	base, ok, err := s.git.TurnRef(sess.Dir, sess.ID)
	if err != nil {
		return Diff{}, fmt.Errorf("review: reading the turn baseline of session %s: %w", sess.ID, err)
	}
	if !ok {
		return Diff{}, ErrNoTurn
	}
	now, err := s.git.SnapshotTree(sess.Dir)
	if err != nil {
		return Diff{}, fmt.Errorf("review: snapshotting session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	raw, err := s.git.DiffTrees(sess.Dir, base, now)
	if err != nil {
		return Diff{}, fmt.Errorf("review: diffing session %s's turn: %w", sess.ID, err)
	}
	return ParseDiff(strings.NewReader(raw))
}

// DropTurn deletes sess's baseline when the session is archived. From
// projectRoot, because the worktree may be removed in the same breath and
// the ref lives in the common repository anyway.
func (s *Source) DropTurn(sess registry.Session, projectRoot string) error {
	if err := s.git.DeleteTurnRef(projectRoot, sess.ID); err != nil {
		return fmt.Errorf("review: deleting the turn baseline of session %s: %w", sess.ID, err)
	}
	return nil
}
