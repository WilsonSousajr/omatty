package review_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/gitdiff"
	"github.com/WilsonSousajr/omatty/internal/service/review"
)

// A diff says which revision its old side is, so the highlighter can read the
// old file whole (#800): the merge-base for a session's diff, the baseline
// tree for a turn's.
func TestSource_aDiffRecordsItsOldSide_issue800(t *testing.T) {
	g := &FakeGit{MergeBaseOut: "abc123", DiffOut: twoFileDiff, TurnTree: "base", SnapshotOut: "now", DiffTreesOut: twoFileDiff}
	src := review.NewSource(g, gitdiff.ParseDiff)

	full, err := src.Load(worktreeSession, "/p/omatty")
	if err != nil || full.Base != "abc123" {
		t.Errorf("Load().Base = %q, %v; want the merge-base", full.Base, err)
	}
	turn, err := src.LoadTurn(turnSess, "/p")
	if err != nil || turn.Base != "base" {
		t.Errorf("LoadTurn().Base = %q, %v; want the baseline tree", turn.Base, err)
	}
}

// OldFile reads a file at a revision in the session's checkout, as a preview
// does from the worktree (#800).
func TestSource_OldFileReadsTheFileAtTheRevision_issue800(t *testing.T) {
	g := &FakeGit{Blobs: map[string]string{"abc123:p.go": "package p\nvar s = `\n"}}

	p, err := review.NewSource(g, gitdiff.ParseDiff).OldFile(worktreeSession, "abc123", "p.go")

	if err != nil || strings.Join(p.Lines, "|") != "package p|var s = `" {
		t.Errorf("OldFile = %q, %v; want the file's lines", p.Lines, err)
	}
	if !strings.Contains(calls(g), "Blob(/wt/parser-fix,abc123,p.go)") {
		t.Errorf("calls = %s, want the blob read in the session's checkout", calls(g))
	}
}

// A file over the preview's bound is not read: OldFile says it was cut, and
// the diff lexes that file's hunks alone (#800).
func TestSource_OldFileOverTheBoundIsTruncated_issue800(t *testing.T) {
	g := &FakeGit{Blobs: map[string]string{}} // no entry: too large

	p, err := review.NewSource(g, gitdiff.ParseDiff).OldFile(worktreeSession, "abc123", "big.go")

	if err != nil || !p.Truncated || p.Lines != nil {
		t.Errorf("OldFile over the bound = %+v, %v; want Truncated and no lines", p, err)
	}
}

// A failed read says which file at which revision.
func TestSource_OldFileFailureNamesTheFile_issue800(t *testing.T) {
	g := &FakeGit{Errs: map[string]error{"Blob": errors.New("no such path")}}

	_, err := review.NewSource(g, gitdiff.ParseDiff).OldFile(worktreeSession, "abc123", "gone.go")

	if err == nil || !strings.Contains(err.Error(), "gone.go") || !strings.Contains(err.Error(), "abc123") {
		t.Errorf("error = %v, want one naming gone.go at abc123", err)
	}
}
