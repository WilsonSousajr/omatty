package review_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/vcs"
)

var turnSess = registry.Session{ID: "s1", Dir: "/wt/s1"}

func TestSource_SnapTurnPointsTheSessionsRefAtItsTree_issue311(t *testing.T) {
	git := &FakeGit{SnapshotOut: "tree1"}

	if err := review.NewSource(git).SnapTurn(turnSess); err != nil {
		t.Fatalf("SnapTurn() error = %v", err)
	}

	want := "SnapshotTree(/wt/s1) SetTurnRef(/wt/s1,s1,tree1)"
	if got := strings.Join(git.Calls, " "); got != want {
		t.Errorf("calls = %s, want %s", got, want)
	}
}

func TestSource_SnapTurnFailureNamesTheSession_issue311(t *testing.T) {
	git := &FakeGit{Errs: map[string]error{"SnapshotTree": errors.New("disk full")}}

	err := review.NewSource(git).SnapTurn(turnSess)

	if err == nil || !strings.Contains(err.Error(), "s1") || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error = %v, want one naming s1 and the cause", err)
	}
}

// With no baseline there is no turn to show, and saying "no changes" would
// be a lie: LoadTurn says so with its own error, and builds nothing.
func TestSource_LoadTurnWithNoBaselineIsErrNoTurn_issue311(t *testing.T) {
	git := &FakeGit{}

	_, err := review.NewSource(git).LoadTurn(turnSess, "/p")

	if !errors.Is(err, review.ErrNoTurn) {
		t.Errorf("error = %v, want ErrNoTurn", err)
	}
	if strings.Contains(strings.Join(git.Calls, " "), "SnapshotTree") {
		t.Errorf("snapshotted with no baseline to diff against: %v", git.Calls)
	}
}

func TestSource_LoadTurnDiffsTheBaselineAgainstTheTreeNow_issue311(t *testing.T) {
	git := &FakeGit{TurnTree: "base", SnapshotOut: "now", DiffTreesOut: twoFileDiff}

	d, err := review.NewSource(git).LoadTurn(turnSess, "/p")

	if err != nil {
		t.Fatalf("LoadTurn() error = %v", err)
	}
	if !strings.Contains(strings.Join(git.Calls, " "), "DiffTrees(/wt/s1,base,now)") {
		t.Errorf("calls = %v, want DiffTrees(/wt/s1,base,now)", git.Calls)
	}
	if len(d.Files) != 2 {
		t.Errorf("parsed %d files, want 2", len(d.Files))
	}
}

// The worktree may already be gone when a session is archived; the ref is
// in the common repository, so it is deleted from the project root.
func TestSource_DropTurnDeletesFromTheProjectRoot_issue311(t *testing.T) {
	git := &FakeGit{}

	if err := review.NewSource(git).DropTurn(turnSess, "/p/omatty"); err != nil {
		t.Fatalf("DropTurn() error = %v", err)
	}
	if got := strings.Join(git.Calls, " "); got != "DeleteTurnRef(/p/omatty,s1)" {
		t.Errorf("calls = %s, want DeleteTurnRef(/p/omatty,s1)", got)
	}
}

// The card decides whether a merged pull request is this work by its head
// commit, so the stat it already polls carries HEAD (#310 final review).
func TestSource_StatCarriesTheHeadCommit_issue310(t *testing.T) {
	g := &FakeGit{Branch: "parser-fix", MergeBaseOut: "abc123", HeadOut: "def456",
		ShortstatOut: vcs.Shortstat{Files: 1, Added: 2, Removed: 1}}

	st, err := review.NewSource(g).Stat(registry.Session{ID: "s1", Dir: "/wt/s1", Branch: "parser-fix", Base: "main"}, "/p")

	if err != nil || st.Head != "def456" {
		t.Errorf("Stat() = %+v, %v; want Head def456", st, err)
	}
}
