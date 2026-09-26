package review_test

import (
	"errors"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// RevertTurn is LoadTurn's mirror: the same baseline, written back instead of
// diffed (#334, on #311's ref).
func TestSourceRevertTurn_RestoresTheTurnBaseline_issue334(t *testing.T) {
	git := &FakeGit{TurnTree: "abc123", DiffTreesOut: twoFileDiff}
	sess := registry.Session{ID: "s1", Dir: "/wt/s1"}

	files, err := review.NewSource(git).RevertTurn(sess)
	if err != nil {
		t.Fatal(err)
	}

	if len(git.Restored) != 1 || git.Restored[0] != "abc123" {
		t.Errorf("restored %v, want the turn baseline abc123", git.Restored)
	}
	if files != 2 {
		t.Errorf("RevertTurn reported %d files, want the 2 the diff names", files)
	}
}

// A session with no baseline has nothing to revert to, and says so with the
// same error the review column already renders as a notice (#311).
func TestSourceRevertTurn_SaysWhenThereIsNoBaseline_issue334(t *testing.T) {
	_, err := review.NewSource(&FakeGit{}).RevertTurn(registry.Session{ID: "s1", Dir: "/wt/s1"})

	if !errors.Is(err, review.ErrNoTurn) {
		t.Errorf("err = %v, want ErrNoTurn", err)
	}
}

// Nothing is written when the restore cannot happen.
func TestSourceRevertTurn_ReportsAFailedRestore_issue334(t *testing.T) {
	git := &FakeGit{TurnTree: "abc123", DiffTreesOut: twoFileDiff,
		Errs: map[string]error{"RestoreTree": errors.New("disk full")}}

	if _, err := review.NewSource(git).RevertTurn(registry.Session{ID: "s1", Dir: "/wt/s1"}); err == nil {
		t.Error("a failed restore was swallowed")
	}
}

// The count the confirmation shows, read before anything is written: how many
// files the turn touched.
func TestSourceTurnFileCount_CountsWhatWouldBeDiscarded_issue334(t *testing.T) {
	git := &FakeGit{TurnTree: "abc123", DiffTreesOut: twoFileDiff}

	n, err := review.NewSource(git).TurnFileCount(registry.Session{ID: "s1", Dir: "/wt/s1"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("TurnFileCount = %d, want 2", n)
	}
	for _, call := range git.Calls {
		if call[:11] == "RestoreTree" {
			t.Error("counting wrote to the working tree")
		}
	}
}
