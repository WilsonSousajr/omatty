package review_test

import (
	"errors"
	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"github.com/WilsonSousajr/omatty/internal/infra/gitdiff"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/service/review"
)

// Stat reads through the same base commit as Load and never lists untracked
// files, so a card can read lower than the review column: the column is the
// truth when opened (#180).
func TestSource_StatCountsTrackedChangesAgainstTheSameBaseAsLoad_issue180(t *testing.T) {
	g := &FakeGit{Branch: "parser-fix", MergeBaseOut: "abc123",
		ShortstatOut: dreview.Shortstat{Files: 2, Added: 12, Removed: 3}}

	st, err := review.NewSource(g, gitdiff.ParseDiff).Stat(worktreeSession, "/p/omatty")

	if err != nil {
		t.Fatal(err)
	}
	want := "CurrentBranch(/wt/parser-fix) MergeBase(/wt/parser-fix,develop) Shortstat(/wt/parser-fix,abc123) Head(/wt/parser-fix)"
	if calls(g) != want {
		t.Errorf("calls = %s\nwant  %s", calls(g), want)
	}
	if st != (review.Stat{Branch: "parser-fix", Added: 12, Removed: 3}) {
		t.Errorf("Stat() = %+v", st)
	}
}

func TestSource_StatFailureNamesTheSessionAndRef_issue180(t *testing.T) {
	g := &FakeGit{Branch: "main", Errs: map[string]error{"Shortstat": errors.New("boom")}}

	_, err := review.NewSource(g, gitdiff.ParseDiff).Stat(session.Session{ID: "s9", Dir: "/p"}, "/p")

	if err == nil || !strings.Contains(err.Error(), "s9") || !strings.Contains(err.Error(), "HEAD") {
		t.Errorf("error = %v, want one naming session s9 and ref HEAD", err)
	}
}

func TestSource_StatBranchFailureNamesTheDirectory_issue180(t *testing.T) {
	g := &FakeGit{Errs: map[string]error{"CurrentBranch": errors.New("not a repository")}}

	_, err := review.NewSource(g, gitdiff.ParseDiff).Stat(session.Session{ID: "s9", Dir: "/gone"}, "/p")

	if err == nil || !strings.Contains(err.Error(), "/gone") {
		t.Errorf("error = %v, want one naming /gone", err)
	}
}

var worktreeSession = session.Session{
	ID: "s2", Dir: "/wt/parser-fix", Branch: "parser-fix", Base: "develop", Worktree: true,
}

func calls(g *FakeGit) string { return strings.Join(g.Calls, " ") }

func TestSource_WorktreeDiffsAgainstTheMergeBaseWithItsBase_issue21(t *testing.T) {
	g := &FakeGit{MergeBaseOut: "abc123", DiffOut: twoFileDiff}

	d, err := review.NewSource(g, gitdiff.ParseDiff).Load(worktreeSession, "/p/omatty")

	if err != nil {
		t.Fatal(err)
	}
	want := "MergeBase(/wt/parser-fix,develop) Diff(/wt/parser-fix,abc123) Untracked(/wt/parser-fix)"
	if calls(g) != want {
		t.Errorf("calls = %s\nwant  %s", calls(g), want)
	}
	if len(d.Files) != 2 {
		t.Errorf("parsed %d files, want 2", len(d.Files))
	}
}

func TestSource_MainCheckoutDiffsAgainstHead_issue21(t *testing.T) {
	g := &FakeGit{}
	sess := session.Session{ID: "s1", Dir: "/p/omatty"}

	if _, err := review.NewSource(g, gitdiff.ParseDiff).Load(sess, "/p/omatty"); err != nil {
		t.Fatal(err)
	}

	if want := "Diff(/p/omatty,HEAD) Untracked(/p/omatty)"; calls(g) != want {
		t.Errorf("calls = %s\nwant  %s", calls(g), want)
	}
}

// A worktree made before M3 has no recorded base; the project root's branch
// stands in.
func TestSource_MissingBaseFallsBackToTheRootsBranch_issue21(t *testing.T) {
	g := &FakeGit{Branch: "main", MergeBaseOut: "def"}
	sess := worktreeSession
	sess.Base = ""

	if _, err := review.NewSource(g, gitdiff.ParseDiff).Load(sess, "/p/omatty"); err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(calls(g), "CurrentBranch(/p/omatty) MergeBase(/wt/parser-fix,main)") {
		t.Errorf("calls = %s, want the root's branch read first and used as the base", calls(g))
	}
}

func TestSource_UntrackedFilesAreAppendedAsAdditions_issue21(t *testing.T) {
	newFile := twoFileDiff[strings.Index(twoFileDiff, "diff --git a/new.txt"):]
	g := &FakeGit{UntrackedOut: []string{"new.txt"}, FileDiffs: map[string]string{"new.txt": newFile}}

	d, err := review.NewSource(g, gitdiff.ParseDiff).Load(session.Session{ID: "s1", Dir: "/p"}, "/p")

	if err != nil {
		t.Fatal(err)
	}
	if len(d.Files) != 1 || d.Files[0].Path != "new.txt" || d.Files[0].Status != dreview.FileAdded {
		t.Errorf("files = %+v, want new.txt as an added file", d.Files)
	}
}

func TestSource_GitFailureNamesTheSessionAndRef(t *testing.T) {
	g := &FakeGit{Err: errors.New("boom")}

	_, err := review.NewSource(g, gitdiff.ParseDiff).Load(session.Session{ID: "s9", Dir: "/p"}, "/p")

	if err == nil || !strings.Contains(err.Error(), "s9") || !strings.Contains(err.Error(), "HEAD") {
		t.Errorf("error = %v, want one naming session s9 and ref HEAD", err)
	}
}

func TestSource_MergeBaseFailureNamesTheBaseBranch(t *testing.T) {
	g := &FakeGit{Err: errors.New("unknown revision")}

	_, err := review.NewSource(g, gitdiff.ParseDiff).Load(worktreeSession, "/p/omatty")

	if err == nil || !strings.Contains(err.Error(), "develop") {
		t.Errorf("error = %v, want one naming the base branch develop", err)
	}
}

func TestSource_UnknownBaseBranchFailureNamesTheProjectRoot(t *testing.T) {
	g := &FakeGit{Err: errors.New("not a repository")}
	sess := worktreeSession
	sess.Base = ""

	_, err := review.NewSource(g, gitdiff.ParseDiff).Load(sess, "/p/omatty")

	if err == nil || !strings.Contains(err.Error(), "/p/omatty") {
		t.Errorf("error = %v, want one naming the project root", err)
	}
}

// The listing succeeded and one file's diff did not, so the error must name
// the file rather than blaming the directory.
func TestSource_UntrackedDiffFailureNamesTheFile(t *testing.T) {
	g := &FakeGit{
		UntrackedOut: []string{"new.txt"},
		Errs:         map[string]error{"UntrackedDiff": errors.New("boom")},
	}

	_, err := review.NewSource(g, gitdiff.ParseDiff).Load(session.Session{ID: "s1", Dir: "/p"}, "/p")

	if err == nil {
		t.Fatal("Load() returned nil after an untracked-diff failure, want an error")
	}
	if !strings.Contains(err.Error(), "new.txt") {
		t.Errorf("error %q does not name the offending file", err)
	}
}

// The diff came back but the listing did not: the untracked half of the
// change is missing, so the load fails rather than showing half a review.
func TestSource_UntrackedListingFailureNamesTheDirectory(t *testing.T) {
	g := &FakeGit{Errs: map[string]error{"Untracked": errors.New("boom")}}

	_, err := review.NewSource(g, gitdiff.ParseDiff).Load(session.Session{ID: "s1", Dir: "/p/omatty"}, "/p/omatty")

	if err == nil {
		t.Fatal("Load() returned nil after a listing failure, want an error")
	}
	if !strings.Contains(err.Error(), "/p/omatty") {
		t.Errorf("error %q does not name the offending directory", err)
	}
}

// A worktree's recorded base is a branch name, and the branch is normally
// deleted once its pull request merges. git then fails merge-base with "Not a
// valid object name", and the session's review could never load again (#684):
// a base that is gone counts as none recorded, so the root's branch stands in.
func TestSource_DeletedBaseFallsBackToTheRootsBranch_issue684(t *testing.T) {
	g := &FakeGit{Branch: "main", MergeBaseOut: "def", DiffOut: twoFileDiff,
		GoneRefs: map[string]bool{"develop": true}}

	d, err := review.NewSource(g, gitdiff.ParseDiff).Load(worktreeSession, "/p/omatty")

	if err != nil {
		t.Fatalf("Load() with a deleted base = %v, want the root's branch to stand in", err)
	}
	want := "MergeBase(/wt/parser-fix,develop) CommitExists(/wt/parser-fix,develop) " +
		"CurrentBranch(/p/omatty) MergeBase(/wt/parser-fix,main) Diff(/wt/parser-fix,def)"
	if !strings.HasPrefix(calls(g), want) {
		t.Errorf("calls = %s\nwant  %s ...", calls(g), want)
	}
	if len(d.Files) != 2 {
		t.Errorf("parsed %d files, want 2", len(d.Files))
	}
}

// The card's diffstat resolves its base through the same path, so it comes
// back too (#684).
func TestSource_StatSurvivesADeletedBase_issue684(t *testing.T) {
	g := &FakeGit{Branch: "parser-fix", MergeBaseOut: "def",
		ShortstatOut: vcs.Shortstat{Added: 4, Removed: 1}, GoneRefs: map[string]bool{"develop": true}}

	st, err := review.NewSource(g, gitdiff.ParseDiff).Stat(worktreeSession, "/p/omatty")

	if err != nil || st.Added != 4 || st.Removed != 1 {
		t.Errorf("Stat() with a deleted base = %+v, %v; want +4 -1, nil", st, err)
	}
}

// Only a base that no longer resolves falls back. A merge-base that fails on a
// base that still exists is a real failure, and must say so rather than
// quietly diffing against some other branch (#684).
func TestSource_MergeBaseFailureOnALiveBaseStillFails_issue684(t *testing.T) {
	g := &FakeGit{Branch: "main", Errs: map[string]error{"MergeBase": errors.New("no merge base")}}

	_, err := review.NewSource(g, gitdiff.ParseDiff).Load(worktreeSession, "/p/omatty")

	if err == nil || !strings.Contains(err.Error(), "develop") {
		t.Errorf("error = %v, want the merge-base failure naming develop", err)
	}
	if strings.Contains(calls(g), "CurrentBranch") {
		t.Errorf("calls = %s, want no fallback to the root's branch", calls(g))
	}
}

// When git cannot even say whether the base exists, the merge-base failure is
// the one reported: guessing would hide what went wrong (#684).
func TestSource_UnanswerableBaseCheckReportsTheMergeBaseFailure_issue684(t *testing.T) {
	g := &FakeGit{Branch: "main", GoneRefs: map[string]bool{"develop": true},
		Errs: map[string]error{"CommitExists": errors.New("not a repository")}}

	_, err := review.NewSource(g, gitdiff.ParseDiff).Load(worktreeSession, "/p/omatty")

	if err == nil || !strings.Contains(err.Error(), "Not a valid object name develop") {
		t.Errorf("error = %v, want the merge-base failure", err)
	}
}
