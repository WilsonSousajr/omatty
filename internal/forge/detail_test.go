package forge_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// detailJSON is `gh issue view --json` output with the fields FoldDetail reads.
func detailJSON(body string, comments ...string) string {
	return `{"number":397,"title":"read one issue or pull request","body":"` + body +
		`","author":{"login":"WilsonSousajr"},"createdAt":"2026-09-25T19:00:00Z","comments":[` +
		strings.Join(comments, ",") + `],"url":"https://github.com/WilsonSousajr/omatty/issues/397"}`
}

func commentJSON(login, body string) string {
	return `{"author":{"login":"` + login + `"},"body":"` + body + `","createdAt":"2026-09-25T20:00:00Z"}`
}

// One call, and the same field set for either kind: an issue and a pull request
// answer to the same names, so one fold serves both.
func TestCLI_ViewIssueRunsOneViewInTheRepoRoot_issue397(t *testing.T) {
	bin, calls := fakeGH(t, detailJSON("the body"), "", 0)
	root := t.TempDir()

	got, err := forge.NewCLIWithBin(bin).ViewIssue(root, 397)

	if err != nil || got.Number != 397 || got.Body != "the body" {
		t.Fatalf("ViewIssue() = %+v, %v; want #397 with its body", got, err)
	}
	out, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	want := "issue view 397 --json number,title,body,author,createdAt,comments,url"
	dir, args, _ := strings.Cut(strings.TrimSpace(string(out)), "|")
	if args != want || !sameDir(t, dir, root) {
		t.Errorf("call was %q in %s, want %q in %s", args, dir, want, root)
	}
}

// A pull request is the other subcommand. Nothing guesses: the tracker knows
// which list its row came from.
func TestCLI_ViewPRRunsTheOtherSubcommand_issue397(t *testing.T) {
	bin, calls := fakeGH(t, detailJSON("pr body"), "", 0)

	if _, err := forge.NewCLIWithBin(bin).ViewPR(t.TempDir(), 400); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if _, args, _ := strings.Cut(strings.TrimSpace(string(out)), "|"); !strings.HasPrefix(args, "pr view 400 ") {
		t.Errorf("call was %q, want `pr view 400`", args)
	}
}

func TestFoldDetail_CarriesBodyAndComments_issue397(t *testing.T) {
	raw := detailJSON("what it is", commentJSON("WilsonSousajr", "the first note"), commentJSON("someone", "the second"))

	got, err := forge.FoldDetail([]byte(raw))

	if err != nil {
		t.Fatalf("FoldDetail() error = %v", err)
	}
	if got.Title != "read one issue or pull request" || got.Author != "WilsonSousajr" {
		t.Errorf("FoldDetail() = %+v, want the title and author", got)
	}
	if want := time.Date(2026, 9, 25, 19, 0, 0, 0, time.UTC); !got.Created.Equal(want) {
		t.Errorf("Created = %v, want %v", got.Created, want)
	}
	if len(got.Comments) != 2 || got.Comments[1].Author != "someone" || got.Comments[0].Body != "the first note" {
		t.Errorf("Comments = %+v, want both in order", got.Comments)
	}
	if got.Truncated {
		t.Error("Truncated = true for a small item")
	}
}

// Bounded, for the reason the preview is (#24): an item is someone else's text
// and can be any size, and the pane must not hold megabytes of it.
func TestFoldDetail_BoundsAnEnormousItem_issue397(t *testing.T) {
	huge := strings.Repeat("a", forge.DetailMax+1)

	got, err := forge.FoldDetail([]byte(detailJSON(huge, commentJSON("someone", "dropped"))))

	if err != nil {
		t.Fatalf("FoldDetail() error = %v", err)
	}
	if len(got.Body) > forge.DetailMax {
		t.Errorf("Body is %d bytes, want it bounded at %d", len(got.Body), forge.DetailMax)
	}
	if !got.Truncated {
		t.Error("Truncated = false after cutting; a short body must not read as the whole one")
	}
	if len(got.Comments) != 0 {
		t.Errorf("Comments = %+v, want none: the bound was already spent on the body", got.Comments)
	}
}

// A comment past the bound is dropped, and the item says so.
func TestFoldDetail_DropsCommentsPastTheBound_issue397(t *testing.T) {
	big := strings.Repeat("b", forge.DetailMax/2)

	got, err := forge.FoldDetail([]byte(detailJSON("small body", commentJSON("a", big), commentJSON("b", big), commentJSON("c", big))))

	if err != nil {
		t.Fatalf("FoldDetail() error = %v", err)
	}
	if len(got.Comments) == 3 || !got.Truncated {
		t.Errorf("kept %d comments, truncated=%v; want the bound to have dropped one", len(got.Comments), got.Truncated)
	}
}

func TestFoldDetail_MalformedJSONIsAnError_issue397(t *testing.T) {
	if _, err := forge.FoldDetail([]byte("{oops")); err == nil {
		t.Error("FoldDetail() error = nil, want one")
	}
}

// b on a row opens the item in the operator's own browser, through their own
// gh: one call in the repository's root, and the number is all gh needs to
// resolve either kind (#398).
func TestCLI_BrowseRunsGhBrowseInTheRepoRoot_issue398(t *testing.T) {
	bin, calls := fakeGH(t, "", "", 0)
	root := t.TempDir()

	if err := forge.NewCLIWithBin(bin).Browse(root, 399); err != nil {
		t.Fatalf("Browse() error = %v", err)
	}

	out, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	dir, args, _ := strings.Cut(strings.TrimSpace(string(out)), "|")
	if args != "browse 399" || !sameDir(t, dir, root) {
		t.Errorf("call was %q in %s, want %q in %s", args, dir, "browse 399", root)
	}
}

func TestCLI_BrowseWithoutGhIsErrNoGH_issue398(t *testing.T) {
	err := forge.NewCLIWithBin(filepath.Join(t.TempDir(), "no-such-gh")).Browse(t.TempDir(), 1)

	if !errors.Is(err, forge.ErrNoGH) {
		t.Errorf("error = %v, want ErrNoGH", err)
	}
}

// A pull request's view carries its checks - each one's name, how it ended and
// how long it took - so the item can list them as the gate lists its steps
// (#433). An issue has no checks, and gh refuses the field for one.
func TestFoldDetail_ReadsAPullRequestsChecks_issue433(t *testing.T) {
	raw := `{"number":400,"title":"t","body":"b","author":{"login":"x"},"comments":[],"statusCheckRollup":[` +
		`{"__typename":"CheckRun","name":"test","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-09-25T19:00:00Z","completedAt":"2026-09-25T19:01:42Z"},` +
		`{"__typename":"CheckRun","name":"lint","status":"COMPLETED","conclusion":"SUCCESS","startedAt":"2026-09-25T19:00:00Z","completedAt":"2026-09-25T19:00:03Z"},` +
		`{"__typename":"StatusContext","context":"ci/e2e","state":"PENDING"}]}`

	got, err := forge.FoldDetail([]byte(raw))

	if err != nil || len(got.Checks) != 3 {
		t.Fatalf("FoldDetail() = %+v, %v; want three checks", got.Checks, err)
	}
	want := []forge.Check{
		{Name: "test", State: forge.CIFailing, Took: 102 * time.Second},
		{Name: "lint", State: forge.CIPassing, Took: 3 * time.Second},
		{Name: "ci/e2e", State: forge.CIRunning},
	}
	for i, w := range want {
		if got.Checks[i] != w {
			t.Errorf("check %d = %+v, want %+v", i, got.Checks[i], w)
		}
	}
}

func TestCLI_ViewPRAsksForItsChecks_issue433(t *testing.T) {
	bin, calls := fakeGH(t, detailJSON("pr body"), "", 0)
	if _, err := forge.NewCLIWithBin(bin).ViewPR(t.TempDir(), 400); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), ",statusCheckRollup") {
		t.Errorf("the pull request's view did not ask for its checks: %q", out)
	}
}
