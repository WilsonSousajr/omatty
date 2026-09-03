package review

import (
	"fmt"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// Source fetches a session's diff through vcs (invariant 4) and parses it.
//
//	src := review.NewSource(vcs.NewCLI())
//	d, err := src.Load(sess, projectRoot)
type Source struct{ git vcs.Git }

// NewSource returns a Source reading through git.
func NewSource(git vcs.Git) *Source { return &Source{git: git} }

// Load returns everything sess changed, committed or not: the working tree
// against the merge-base with the session's base branch, plus untracked files
// as additions (#21). A main-checkout session has no base branch and diffs
// against HEAD. projectRoot is the fallback base for worktrees created before
// the base was recorded.
func (s *Source) Load(sess registry.Session, projectRoot string) (Diff, error) {
	ref, err := s.baseCommit(sess, projectRoot)
	if err != nil {
		return Diff{}, err
	}
	raw, err := s.git.Diff(sess.Dir, ref)
	if err != nil {
		return Diff{}, fmt.Errorf("review: diffing session %s against %s: %w", sess.ID, ref, err)
	}
	extra, err := s.untrackedDiffs(sess.Dir)
	if err != nil {
		return Diff{}, err
	}
	return ParseDiff(strings.NewReader(raw + extra))
}

// baseCommit is HEAD for a main-checkout session, else the merge-base with the
// recorded base branch, or with the project root's current branch when none
// was recorded.
func (s *Source) baseCommit(sess registry.Session, projectRoot string) (string, error) {
	if sess.Branch == "" {
		return "HEAD", nil
	}
	base := sess.Base
	if base == "" {
		cur, err := s.git.CurrentBranch(projectRoot)
		if err != nil {
			return "", fmt.Errorf("review: session %s has no base branch and %q reports none: %w",
				sess.ID, projectRoot, err)
		}
		base = cur
	}
	commit, err := s.git.MergeBase(sess.Dir, base)
	if err != nil {
		return "", fmt.Errorf("review: merge-base of session %s with %q: %w", sess.ID, base, err)
	}
	return commit, nil
}

// untrackedDiffs renders every untracked file as an all-additions diff, so a
// file claude has written but not committed is reviewable like any other.
func (s *Source) untrackedDiffs(dir string) (string, error) {
	paths, err := s.git.Untracked(dir)
	if err != nil {
		return "", fmt.Errorf("review: listing untracked files in %q: %w", dir, err)
	}
	var b strings.Builder
	for _, p := range paths {
		d, err := s.git.UntrackedDiff(dir, p)
		if err != nil {
			return "", fmt.Errorf("review: diffing untracked %q in %q: %w", p, dir, err)
		}
		b.WriteString(d)
	}
	return b.String(), nil
}
