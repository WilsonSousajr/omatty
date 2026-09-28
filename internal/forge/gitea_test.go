package forge_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// giteaRoutes maps a request's path and query to its recorded fixture, for
// the fake tea and the fake API alike.
var giteaRoutes = []struct{ match, file string }{
	{"pulls?state=open", "pulls-open.json"},
	{"pulls?state=closed", "pulls-closed.json"},
	{"/status", "status.json"},
	{"issues?state=open", "issues.json"},
	{"issues/14587/comments", "pull-comments.json"},
	{"issues/2809/comments", "issue-comments.json"},
	{"pulls/14587", "pull.json"},
	{"issues/2809", "issue.json"},
}

func giteaFixture(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "testdata", "forge", "gitea", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// fakeTea is a tea holding one login, for logins, whose `api --include`
// answers each path from the fixtures with its status line on stderr, as the
// real one does - and answers a path it has no fixture for with a 404 body
// and exit 0, as the real one does too.
func fakeTea(t *testing.T, logins string) (bin, calls string) {
	t.Helper()
	dir := t.TempDir()
	calls = filepath.Join(dir, "calls")
	var cases strings.Builder
	for _, r := range giteaRoutes {
		cases.WriteString("  *'" + r.match + "'*) echo 'HTTP/1.1 200 OK' >&2; cat '" + giteaFixture(t, r.file) + "' ;;\n")
	}
	script := "#!/bin/sh\n" +
		`printf '%s\n' "$*" >> '` + calls + "'\n" +
		`if [ "$1 $2" = "logins list" ]; then echo '` + logins + `'; exit 0; fi` + "\n" +
		`case "$*" in` + "\n" + cases.String() +
		`  *) echo 'HTTP/1.1 404 Not Found' >&2; echo '{"message":"not found"}' ;;` + "\n" + "esac\n"
	bin = filepath.Join(dir, "tea")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin, calls
}

const codebergLogin = `[{"name":"codeberg","url":"https://codeberg.org","ssh_host":"codeberg.org","user":"someone","default":"true"}]`

// teaRouter is a Router over forgejo/forgejo on Codeberg with tea as the fake.
func teaRouter(t *testing.T, logins string) (*forge.Router, string) {
	bin, calls := fakeTea(t, logins)
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://codeberg.org/forgejo/forgejo.git"}).url},
		Bins:    map[forge.Kind]string{forge.KindGitea: bin},
	}), calls
}

// Codeberg's pull requests reach the card through tea: fork, head, draft,
// conflict and a CI verdict from the head's combined status (#458).
func TestGitea_ListsPullRequestsWithTheirCI_issue458(t *testing.T) {
	r, _ := teaRouter(t, codebergLogin)

	prs, err := r.ListPRs(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}
	pr, _ := prNumbered(prs, 14587)
	want := forge.PR{Number: 14587, Title: pr.Title, Branch: "runner-refs/heads/main", State: forge.Open,
		CI: forge.CIRunning, Fork: true, Head: "ffd52b9a8d6144520b713dfc75c4cad687d5f68e", Updated: pr.Updated}
	if pr != want || pr.Title == "" {
		t.Errorf("#14587 = %+v\nwant a fork on runner-refs/heads/main whose checks are pending", pr)
	}
}

// The recently merged come too, with their times (#332).
func TestGitea_ListsTheRecentlyMerged_issue458(t *testing.T) {
	r, _ := teaRouter(t, codebergLogin)
	prs, err := r.ListPRs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	merged := 0
	for _, pr := range prs {
		if pr.State == forge.Merged && !pr.MergedAt.IsZero() {
			merged++
		}
	}
	if merged != 3 {
		t.Errorf("read %d merged pull requests with their times, want 3", merged)
	}
}

// A commit status maps to the card's CI mark; unknown is running, never passing.
func TestGitea_StatusIsTheCIMark_issue458(t *testing.T) {
	for status, want := range map[string]forge.CIState{
		"success": forge.CIPassing, "skipped": forge.CIPassing, "warning": forge.CIPassing,
		"failure": forge.CIFailing, "error": forge.CIFailing,
		"pending": forge.CIRunning, "something-new": forge.CIRunning,
	} {
		if got := forge.GiteaCI(status); got != want {
			t.Errorf("status %q = %v, want %v", status, got, want)
		}
	}
}

// Issues, and one in full with its comments.
func TestGitea_ListsIssuesAndReadsOneInFull_issue458(t *testing.T) {
	r, _ := teaRouter(t, codebergLogin)
	root := t.TempDir()

	issues, err := r.ListIssues(root)
	if err != nil || len(issues) != 3 || issues[0].Number != 14588 || len(issues[0].Labels) < 2 {
		t.Fatalf("issues = %+v, %v; want the three recorded, labelled", issues, err)
	}
	d, err := r.ViewIssue(root, 2809)
	if err != nil || d.Author != "SinTan1729" || len(d.Comments) != 3 || d.Comments[0].Author != "oliverpool" {
		t.Errorf("detail = %+v, %v; want #2809 by SinTan1729 with three comments", d, err)
	}
}

// A pull request in full carries its head's statuses as its checks.
func TestGitea_ReadsAPullRequestWithItsChecks_issue458(t *testing.T) {
	r, _ := teaRouter(t, codebergLogin)

	d, err := r.ViewPR(t.TempDir(), 14587)

	if err != nil || d.Number != 14587 || len(d.Checks) != 15 {
		t.Errorf("detail = %+v, %v; want #14587 with its 15 statuses as checks", d, err)
	}
}

// tea is asked through the login that names the project's host, with the
// status line it prints on --include.
func TestGitea_TeaIsAskedThroughTheHostsLogin_issue458(t *testing.T) {
	r, calls := teaRouter(t, codebergLogin)

	_, _ = r.ListIssues(t.TempDir())

	b, _ := os.ReadFile(calls)
	if !strings.Contains(string(b), "api --login codeberg --include /repos/forgejo/forgejo/issues?state=open") {
		t.Errorf("tea was called:\n%s\nwant api through the codeberg login", b)
	}
}

// tea prints a 404 and exits 0; the status line is what says so.
func TestGitea_TeasStatusLineIsClassified_issue458(t *testing.T) {
	r, _ := teaRouter(t, codebergLogin)

	_, err := r.ViewIssue(t.TempDir(), 1)

	if err == nil || !errors.Is(err, forge.ErrNotFound) {
		t.Errorf("error = %v, want not found", err)
	}
}

// Codeberg is named as itself; any other Gitea or Forgejo host as Gitea.
func TestGitea_NamesCodebergAsItself_issue458(t *testing.T) {
	r, _ := teaRouter(t, codebergLogin)
	root := t.TempDir()
	_, _ = r.ListIssues(root)

	if got := r.Label(root); got.Forge != "Codeberg" || got.Short != "PR" || got.Sigil != "#" {
		t.Errorf("Label = %+v, want Codeberg's pull requests", got)
	}
}

// b opens the item's own page.
func TestGitea_BrowseOpensTheItemsPage_issue458(t *testing.T) {
	var opened []string
	bin, _ := fakeTea(t, codebergLogin)
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@codeberg.org:forgejo/forgejo.git"}).url},
		Bins:    map[forge.Kind]string{forge.KindGitea: bin},
		Open:    func(url string) error { opened = append(opened, url); return nil },
	})

	_ = r.BrowsePR(t.TempDir(), 14587)
	_ = r.BrowseIssue(t.TempDir(), 2809)

	want := []string{"https://codeberg.org/forgejo/forgejo/pulls/14587", "https://codeberg.org/forgejo/forgejo/issues/2809"}
	if !reflect.DeepEqual(opened, want) {
		t.Errorf("opened %v, want %v", opened, want)
	}
}

// Without tea, or with a tea that has no login for the host, there is no CLI
// path; #459 adds the REST one. Until then it is a missing tool.
func TestGitea_ATeaWithoutALoginForTheHostIsNotUsed_issue458_issue586(t *testing.T) {
	r, calls := teaRouter(t, `[]`)

	_, err := r.ListPRs(t.TempDir())

	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "tea" {
		t.Errorf("error = %v, want tea missing", err)
	}
	// tea is installed; what it lacks is a login for this host (#586).
	if missing != nil && (missing.NoLoginFor != "codeberg.org" || !strings.Contains(err.Error(), "tea has no login for codeberg.org")) {
		t.Errorf("error = %v (%+v), want tea's missing login for codeberg.org named", err, missing)
	}
	if b, _ := os.ReadFile(calls); strings.Contains(string(b), "api") {
		t.Errorf("tea api was run with no login for the host:\n%s", b)
	}
}
