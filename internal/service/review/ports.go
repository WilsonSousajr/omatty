package review

import (
	"io"

	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
)

// Git is the slice of git a Source reads and writes through: a session's diff
// and stat against its base, its untracked files, the turn baseline it
// snapshots and restores, and the attribute that marks a file generated.
// Declared here, by its consumer, since migration step 5.8 (ADR 0001's
// ports, #653); internal/infra/vcs's CLI implements it.
//
//	src := review.NewSource(vcs.NewCLI(), gitdiff.ParseDiff)
type Git interface {
	CurrentBranch(dir string) (string, error)
	CommitsOnBranch(repoRoot, base, branch string) (int, error)
	MergeBase(dir, ref string) (string, error)
	CommitExists(dir, ref string) (bool, error)
	Diff(dir, commit string) (string, error)
	Shortstat(dir, commit string) (dreview.Shortstat, error)
	Untracked(dir string) ([]string, error)
	UntrackedDiff(dir, path string) (string, error)
	SnapshotTree(dir string) (string, error)
	SetTurnRef(dir, id, tree string) error
	TurnRef(dir, id string) (string, bool, error)
	DeleteTurnRef(dir, id string) error
	DiffTrees(dir, from, to string) (string, error)
	Head(dir string) (string, error)
	Attr(dir, attr string, paths []string) (map[string]bool, error)
	RestoreTree(dir, tree string) error
}

// ParseFunc parses git's unified diff output into a Diff. Injected, because
// go-gitdiff is reachable only through internal/infra/gitdiff (migration step
// 5.8, #653): cmd passes gitdiff.ParseDiff.
type ParseFunc func(r io.Reader) (dreview.Diff, error)
