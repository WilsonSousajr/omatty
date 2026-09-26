package review_test

import (
	"errors"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/vcs"
)

var shipSession = registry.Session{ID: "s1", Dir: "/wt/s1", Branch: "feat/a", Base: "develop"}

// #331 pushes commits, and the gate verified a working tree. A session with
// uncommitted work would open a pull request that differs from what was
// verified, so it has to be countable.
func TestShippable_CountsUncommittedAndUntrackedTogether_issue331(t *testing.T) {
	git := &FakeGit{ShortstatOut: vcs.Shortstat{Files: 2}, UntrackedOut: []string{"new.go"}, Commits: 3}

	got, err := review.NewSource(git).Shippable(shipSession, "/p/omatty")
	if err != nil {
		t.Fatal(err)
	}

	if got.Uncommitted != 3 {
		t.Errorf("Uncommitted = %d, want 2 modified plus 1 untracked", got.Uncommitted)
	}
	if got.Commits != 3 {
		t.Errorf("Commits = %d, want 3", got.Commits)
	}
}

// Clean is zero, and it is measured against HEAD rather than against the base -
// otherwise every committed change would read as uncommitted and nothing could
// ever ship.
func TestShippable_MeasuresAgainstHead_issue331(t *testing.T) {
	git := &FakeGit{Commits: 1}

	got, err := review.NewSource(git).Shippable(shipSession, "/p/omatty")
	if err != nil {
		t.Fatal(err)
	}

	if got.Uncommitted != 0 {
		t.Errorf("Uncommitted = %d, want 0 for a clean tree", got.Uncommitted)
	}
	var asked bool
	for _, call := range git.Calls {
		if call == "Shortstat(/wt/s1,HEAD)" {
			asked = true
		}
	}
	if !asked {
		t.Errorf("calls = %v, want a shortstat against HEAD", git.Calls)
	}
}

// A main-checkout session has no branch of its own and no base it diverged
// from. Reporting no commits is what refuses the ship, rather than opening a
// pull request from a branch onto itself.
func TestShippable_AMainCheckoutSessionHasNothingToShip_issue331(t *testing.T) {
	git := &FakeGit{Commits: 7}

	got, err := review.NewSource(git).Shippable(registry.Session{ID: "s1", Dir: "/p/omatty"}, "/p/omatty")
	if err != nil {
		t.Fatal(err)
	}

	if got.Commits != 0 {
		t.Errorf("Commits = %d, want none for a session with no branch of its own", got.Commits)
	}
}

func TestShippable_ReportsGitFailing_issue331(t *testing.T) {
	git := &FakeGit{Errs: map[string]error{"Untracked": errors.New("git is unwell")}}

	if _, err := review.NewSource(git).Shippable(shipSession, "/p/omatty"); err == nil {
		t.Error("a failing git read was swallowed")
	}
}
