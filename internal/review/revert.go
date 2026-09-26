package review

import (
	"fmt"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// RevertTurn puts sess's working tree back to where the turn started and
// reports how many files it changed (#334).
//
//	files, err := src.RevertTurn(sess)
//
// The mirror of LoadTurn, on the same baseline #311 already records: that issue
// needed a tree snapped at each turn boundary to scope review to "since the
// last turn", and the same ref is exactly what a revert restores to. So this
// adds a restore and no new state.
//
// The count is read *before* the restore, because afterwards there is nothing
// left to count - and it is the number the confirmation showed the operator, so
// what they agreed to is what they are told happened.
//
// Destructive, and the caller owns the confirming: this function does what it is
// asked. Telling the session what happened is deliberately not here either -
// sending anything is S's job and needs a person (#334).
func (s *Source) RevertTurn(sess registry.Session) (int, error) {
	base, files, err := s.turnBaseline(sess)
	if err != nil {
		return 0, err
	}
	if err := s.git.RestoreTree(sess.Dir, base); err != nil {
		return 0, fmt.Errorf("review: reverting session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	return files, nil
}

// TurnFileCount is how many files the turn has changed so far: what a revert
// would discard, for the confirmation to name. It writes nothing.
//
//	n, err := src.TurnFileCount(sess)
func (s *Source) TurnFileCount(sess registry.Session) (int, error) {
	_, files, err := s.turnBaseline(sess)
	return files, err
}

// turnBaseline is the session's baseline tree and how many files it differs
// from the working tree by.
func (s *Source) turnBaseline(sess registry.Session) (string, int, error) {
	d, err := s.LoadTurn(sess, "")
	if err != nil {
		return "", 0, err
	}
	base, ok, err := s.git.TurnRef(sess.Dir, sess.ID)
	if err != nil {
		return "", 0, fmt.Errorf("review: reading the turn baseline of session %s: %w", sess.ID, err)
	}
	if !ok {
		return "", 0, ErrNoTurn
	}
	return base, len(d.Files), nil
}
