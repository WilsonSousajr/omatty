package review

import (
	"fmt"
	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"io"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
)

// Source fetches a session's diff through vcs (invariant 4) and parses it.
//
//	src := review.NewSource(vcs.NewCLI(), gitdiff.ParseDiff)
//	d, err := src.Load(sess, projectRoot)
type Source struct {
	git   Git
	parse ParseFunc
	// head reads the start of a worktree file for the generated-file sniff
	// (#338). Injected, because opening a file is infra's business (migration
	// step 5.8, #653); nil reads nothing, so no header says "generated" -
	// the safe direction to be wrong in.
	head HeadFunc
}

// HeadFunc reads at most limit bytes from the start of rel under dir.
// internal/infra/fsread's Head is the real one.
type HeadFunc func(dir, rel string, limit int64) ([]byte, error)

// NewSource returns a Source reading through git and parsing with parse.
func NewSource(git Git, parse ParseFunc) *Source { return &Source{git: git, parse: parse} }

// WithHeads returns s reading file heads through head.
//
//	src := review.NewSource(vcs.NewCLI(), gitdiff.ParseDiff).WithHeads(fsread.Head)
func (s *Source) WithHeads(head HeadFunc) *Source {
	cp := *s
	cp.head = head
	return &cp
}

// Load returns everything sess changed, committed or not: the working tree
// against the merge-base with the session's base branch, plus untracked files
// as additions (#21). A main-checkout session has no base branch and diffs
// against HEAD. projectRoot is the fallback base for worktrees created before
// the base was recorded.
func (s *Source) Load(sess session.Session, projectRoot string) (dreview.Diff, error) {
	ref, err := s.baseCommit(sess, projectRoot)
	if err != nil {
		return dreview.Diff{}, err
	}
	raw, err := s.git.Diff(sess.Dir, ref)
	if err != nil {
		return dreview.Diff{}, fmt.Errorf("review: diffing session %s against %s: %w", sess.ID, ref, err)
	}
	extra, err := s.untrackedDiffs(sess.Dir)
	if err != nil {
		return dreview.Diff{}, err
	}
	// Two readers, not raw+extra: the concatenation allocated a second copy
	// of the whole diff, and a session that touches a lockfile or generated
	// code makes that copy large. ParseDiff only ever reads forward.
	return s.parse(io.MultiReader(strings.NewReader(raw), strings.NewReader(extra)))
}

// baseCommit is HEAD for a main-checkout session, else the merge-base with the
// recorded base branch, or with the project root's current branch when none
// was recorded or the recorded one has since been deleted (#684).
func (s *Source) baseCommit(sess session.Session, projectRoot string) (string, error) {
	if sess.Branch == "" {
		return "HEAD", nil
	}
	if sess.Base == "" {
		return s.rootBranchBase(sess, projectRoot)
	}
	commit, err := s.mergeBase(sess, sess.Base)
	if err == nil || !s.baseGone(sess) {
		return commit, err
	}
	return s.rootBranchBase(sess, projectRoot)
}

// baseGone reports whether sess's recorded base no longer resolves: the usual
// end of a branch whose pull request merged, after which merge-base fails on
// it for good (#684). Asked only once merge-base has failed, so the card's
// poll pays nothing for it. A check git cannot answer is not "gone": the
// caller keeps the failure it already has rather than guess.
func (s *Source) baseGone(sess session.Session) bool {
	exists, err := s.git.CommitExists(sess.Dir, sess.Base)
	return err == nil && !exists
}

// rootBranchBase is the merge-base with the project root's current branch,
// the stand-in for a base that was never recorded or no longer exists.
func (s *Source) rootBranchBase(sess session.Session, projectRoot string) (string, error) {
	cur, err := s.git.CurrentBranch(projectRoot)
	if err != nil {
		return "", fmt.Errorf("review: session %s has no base branch and %q reports none: %w",
			sess.ID, projectRoot, err)
	}
	return s.mergeBase(sess, cur)
}

// mergeBase is where ref and sess's HEAD diverged.
func (s *Source) mergeBase(sess session.Session, ref string) (string, error) {
	commit, err := s.git.MergeBase(sess.Dir, ref)
	if err != nil {
		return "", fmt.Errorf("review: merge-base of session %s with %q: %w", sess.ID, ref, err)
	}
	return commit, nil
}

// Stat is what a session's sidebar card shows about its checkout (#180): the
// branch, and the lines added and removed against the same base Load diffs
// against. Tracked changes only - Load also renders untracked files as
// additions - so a card can read lower than the review column, and the
// column is the truth when opened.
type Stat struct {
	Branch         string
	Added, Removed int
	// Head is the checked-out commit, empty when it could not be read; the
	// card matches a merged pull request against it (#310).
	Head string
}

// Stat reads sess's branch and shortstat through baseCommit, so the base
// resolution lives once (#180).
//
//	st, err := src.Stat(sess, projectRoot)
func (s *Source) Stat(sess session.Session, projectRoot string) (Stat, error) {
	branch, err := s.git.CurrentBranch(sess.Dir)
	if err != nil {
		return Stat{}, fmt.Errorf("review: branch of session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	ref, err := s.baseCommit(sess, projectRoot)
	if err != nil {
		return Stat{}, err
	}
	short, err := s.git.Shortstat(sess.Dir, ref)
	if err != nil {
		return Stat{}, fmt.Errorf("review: shortstat of session %s against %s: %w", sess.ID, ref, err)
	}
	// A HEAD that will not read costs only the merged-PR match, so it is not
	// worth failing the whole card over.
	head, _ := s.git.Head(sess.Dir)
	return Stat{Branch: branch, Added: short.Added, Removed: short.Removed, Head: head}, nil
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
