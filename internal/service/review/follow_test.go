package review_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/service/review"
)

// followGit is a repository /r with a linked worktree /r/.worktrees/x on
// branch feat-x, and an unrelated repository /other.
func followGit() *FakeGit {
	return &FakeGit{
		Branch: "feat-x",
		Roots: map[string]string{
			"/r": "/r", "/r/internal": "/r",
			"/r/.worktrees/x": "/r/.worktrees/x", "/r/.worktrees/x/internal/ui": "/r/.worktrees/x",
			"/other/src": "/other",
		},
		Mains: map[string]string{"/r": "/r", "/r/.worktrees/x": "/r", "/other": "/other"},
	}
}

// claude moved into a worktree of the session's own repository: the review
// reads that checkout, at its top level and on its branch (#659).
func TestFollow_aWorktreeOfTheSameRepository_issue659(t *testing.T) {
	src := review.NewSource(followGit(), nil)
	sess := session.Session{ID: "s1", Dir: "/r"}

	got, err := src.Follow(sess, "/r/.worktrees/x/internal/ui")

	if err != nil || got.Dir != "/r/.worktrees/x" || got.Branch != "feat-x" {
		t.Errorf("Follow = %+v, %v; want Dir /r/.worktrees/x on feat-x", got, err)
	}
}

// A subdirectory of the session's own checkout is the same checkout: the
// review does not move, however often claude cds (#659).
func TestFollow_aSubdirectoryIsTheSameCheckout_issue659(t *testing.T) {
	src := review.NewSource(followGit(), nil)
	sess := session.Session{ID: "s1", Dir: "/r", Branch: "main-work"}

	got, err := src.Follow(sess, "/r/internal")

	if err != nil || got != sess {
		t.Errorf("Follow = %+v, %v; want the session unchanged", got, err)
	}
}

// Another repository is not followed: the gate and ship would act on code
// the session does not own (#659).
func TestFollow_anotherRepositoryIsNotFollowed_issue659(t *testing.T) {
	src := review.NewSource(followGit(), nil)
	sess := session.Session{ID: "s1", Dir: "/r"}

	got, err := src.Follow(sess, "/other/src")

	if err != nil || got != sess {
		t.Errorf("Follow = %+v, %v; want the session unchanged", got, err)
	}
}

// Back from a worktree into the main checkout, the review reads it against
// HEAD, as a main-checkout session does: no branch (#659).
func TestFollow_theMainCheckoutHasNoBranch_issue659(t *testing.T) {
	src := review.NewSource(followGit(), nil)
	sess := session.Session{ID: "s1", Dir: "/r/.worktrees/x", Branch: "feat-x", Base: "develop"}

	got, err := src.Follow(sess, "/r/internal")

	if err != nil || got.Dir != "/r" || got.Branch != "" {
		t.Errorf("Follow = %+v, %v; want Dir /r with no branch", got, err)
	}
}

// A directory git cannot place says which one, and what was expected.
func TestFollow_aDirectoryOutsideAnyCheckoutIsAnError_issue659(t *testing.T) {
	g := followGit()
	g.Errs = map[string]error{"RepoRoot": errors.New("not a git repository")}
	src := review.NewSource(g, nil)

	_, err := src.Follow(session.Session{ID: "s1", Dir: "/r"}, "/tmp/scratch")

	if err == nil || !strings.Contains(err.Error(), `"/tmp/scratch"`) {
		t.Errorf("err = %v, want it to name the directory", err)
	}
}
