package review_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
)

var worktreeSession = registry.Session{
	ID: "s2", Dir: "/wt/parser-fix", Branch: "parser-fix", Base: "develop", Worktree: true,
}

func calls(g *FakeGit) string { return strings.Join(g.Calls, " ") }

func TestSource_WorktreeDiffsAgainstTheMergeBaseWithItsBase_issue21(t *testing.T) {
	g := &FakeGit{MergeBaseOut: "abc123", DiffOut: twoFileDiff}

	d, err := review.NewSource(g).Load(worktreeSession, "/p/omatty")

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
	sess := registry.Session{ID: "s1", Dir: "/p/omatty"}

	if _, err := review.NewSource(g).Load(sess, "/p/omatty"); err != nil {
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

	if _, err := review.NewSource(g).Load(sess, "/p/omatty"); err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(calls(g), "CurrentBranch(/p/omatty) MergeBase(/wt/parser-fix,main)") {
		t.Errorf("calls = %s, want the root's branch read first and used as the base", calls(g))
	}
}

func TestSource_UntrackedFilesAreAppendedAsAdditions_issue21(t *testing.T) {
	newFile := twoFileDiff[strings.Index(twoFileDiff, "diff --git a/new.txt"):]
	g := &FakeGit{UntrackedOut: []string{"new.txt"}, FileDiffs: map[string]string{"new.txt": newFile}}

	d, err := review.NewSource(g).Load(registry.Session{ID: "s1", Dir: "/p"}, "/p")

	if err != nil {
		t.Fatal(err)
	}
	if len(d.Files) != 1 || d.Files[0].Path != "new.txt" || d.Files[0].Status != review.FileAdded {
		t.Errorf("files = %+v, want new.txt as an added file", d.Files)
	}
}

func TestSource_GitFailureNamesTheSessionAndRef(t *testing.T) {
	g := &FakeGit{Err: errors.New("boom")}

	_, err := review.NewSource(g).Load(registry.Session{ID: "s9", Dir: "/p"}, "/p")

	if err == nil || !strings.Contains(err.Error(), "s9") || !strings.Contains(err.Error(), "HEAD") {
		t.Errorf("error = %v, want one naming session s9 and ref HEAD", err)
	}
}

func TestSource_MergeBaseFailureNamesTheBaseBranch(t *testing.T) {
	g := &FakeGit{Err: errors.New("unknown revision")}

	_, err := review.NewSource(g).Load(worktreeSession, "/p/omatty")

	if err == nil || !strings.Contains(err.Error(), "develop") {
		t.Errorf("error = %v, want one naming the base branch develop", err)
	}
}

func TestSource_UnknownBaseBranchFailureNamesTheProjectRoot(t *testing.T) {
	g := &FakeGit{Err: errors.New("not a repository")}
	sess := worktreeSession
	sess.Base = ""

	_, err := review.NewSource(g).Load(sess, "/p/omatty")

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

	_, err := review.NewSource(g).Load(registry.Session{ID: "s1", Dir: "/p"}, "/p")

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

	_, err := review.NewSource(g).Load(registry.Session{ID: "s1", Dir: "/p/omatty"}, "/p/omatty")

	if err == nil {
		t.Fatal("Load() returned nil after a listing failure, want an error")
	}
	if !strings.Contains(err.Error(), "/p/omatty") {
		t.Errorf("error %q does not name the offending directory", err)
	}
}
