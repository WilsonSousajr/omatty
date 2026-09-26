package forge_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// issue builds one element of `gh issue list --json` with the fields
// ListIssues asks for.
func issue(number int, title, labels, assignees string) string {
	return `{"number":` + strconv.Itoa(number) + `,"title":"` + title +
		`","labels":[` + labels + `],"assignees":[` + assignees +
		`],"author":{"login":"WilsonSousajr"},"updatedAt":"2026-09-25T19:39:36Z",` +
		`"url":"https://github.com/WilsonSousajr/omatty/issues/` + strconv.Itoa(number) + `"}`
}

func label(name string) string { return `{"name":"` + name + `"}` }

func user(login string) string { return `{"login":"` + login + `"}` }

func foldOneIssue(t *testing.T, elem string) forge.Issue {
	t.Helper()
	issues, err := forge.FoldIssues([]byte("[" + elem + "]"))
	if err != nil {
		t.Fatalf("FoldIssues() error = %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("FoldIssues() = %d issues, want 1", len(issues))
	}
	return issues[0]
}

// One call for the whole project, in its root, asking for exactly the fields
// FoldIssues reads - so gh resolves the repository from the checkout's own
// remote and the operator's own authentication. Never one call per issue:
// that is what tripped GitHub's secondary rate limit before (#358).
func TestCLI_ListIssuesRunsOneListInTheRepoRoot_issue393(t *testing.T) {
	bin, calls := fakeGH(t, "["+issue(399, "filter the tracker", label("feat"), "")+"]", "", 0)
	root := t.TempDir()

	issues, err := forge.NewCLIWithBin(bin).ListIssues(root)

	if err != nil || len(issues) != 1 || issues[0].Number != 399 {
		t.Fatalf("ListIssues() = %+v, %v; want #399", issues, err)
	}
	got, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	want := "issue list --state open --limit 100 --json number,title,labels,assignees,author,updatedAt,url"
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 1 {
		t.Fatalf("gh was called %d times, want 1:\n%s", len(lines), got)
	}
	dir, args, _ := strings.Cut(lines[0], "|")
	if args != want || !sameDir(t, dir, root) {
		t.Errorf("call was %q in %s, want %q in %s", args, dir, want, root)
	}
}

// A row is a number, a label, a title and an age, and `a` pastes the url, so
// the fold carries exactly those.
func TestFoldIssues_CarriesTheRowsFields_issue393(t *testing.T) {
	got := foldOneIssue(t, issue(396, "the tracker view", label("feat")+","+label("area:ui"), user("WilsonSousajr")))

	want := forge.Issue{
		Number:   396,
		Title:    "the tracker view",
		Labels:   []string{"feat", "area:ui"},
		Assignee: "WilsonSousajr",
		Author:   "WilsonSousajr",
		Updated:  time.Date(2026, 9, 25, 19, 39, 36, 0, time.UTC),
		URL:      "https://github.com/WilsonSousajr/omatty/issues/396",
	}
	if got.Number != want.Number || got.Title != want.Title || got.URL != want.URL {
		t.Errorf("FoldIssues() = %+v, want %+v", got, want)
	}
	if strings.Join(got.Labels, ",") != strings.Join(want.Labels, ",") {
		t.Errorf("labels = %v, want %v", got.Labels, want.Labels)
	}
	if got.Assignee != want.Assignee || got.Author != want.Author {
		t.Errorf("assignee/author = %q/%q, want %q/%q", got.Assignee, got.Author, want.Assignee, want.Author)
	}
	if !got.Updated.Equal(want.Updated) {
		t.Errorf("updated = %v, want %v", got.Updated, want.Updated)
	}
}

// Nobody assigned is the common case in a one-person repository, and it must
// fold to an empty name rather than panicking on an empty array.
func TestFoldIssues_AnUnassignedIssueHasNoAssignee_issue393(t *testing.T) {
	if got := foldOneIssue(t, issue(399, "filter the tracker", "", "")); got.Assignee != "" {
		t.Errorf("assignee = %q, want empty", got.Assignee)
	}
}

func TestFoldIssues_MalformedJSONIsAnError_issue393(t *testing.T) {
	if _, err := forge.FoldIssues([]byte("not json")); err == nil {
		t.Error("FoldIssues() error = nil, want one")
	}
}

// The same three answers ListPRs gives, because they are the same checkout's:
// no gh at all, and a checkout gh cannot map to a GitHub repository.
func TestCLI_ListIssuesWithoutGhIsErrNoGH_issue393(t *testing.T) {
	_, err := forge.NewCLIWithBin(filepath.Join(t.TempDir(), "no-such-gh")).ListIssues(t.TempDir())

	if !errors.Is(err, forge.ErrNoGH) {
		t.Errorf("error = %v, want ErrNoGH", err)
	}
}

func TestCLI_ListIssuesRecognisesARepoThatIsNotOnGitHub_issue393(t *testing.T) {
	bin, _ := fakeGH(t, "", "no git remotes found", 1)

	_, err := forge.NewCLIWithBin(bin).ListIssues(t.TempDir())

	if !errors.Is(err, forge.ErrNotGitHub) {
		t.Errorf("error = %v, want ErrNotGitHub", err)
	}
}

// The failure names the call, not "pr list", now that two lists share one
// runner: a message naming the wrong subcommand sends the reader to the wrong
// place.
func TestCLI_ListIssuesFailureNamesTheCall_issue393(t *testing.T) {
	bin, _ := fakeGH(t, "", "HTTP 401: Bad credentials", 1)

	_, err := forge.NewCLIWithBin(bin).ListIssues(t.TempDir())

	if err == nil || !strings.Contains(err.Error(), "issue list") {
		t.Errorf("error = %v, want it to name `issue list`", err)
	}
}
