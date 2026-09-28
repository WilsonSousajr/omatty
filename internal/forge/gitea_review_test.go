package forge_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// The answers #458's review needed: a unit that is off, a repository that is
// there, and a list of two unmergeable pull requests, one a draft.
var (
	notFound  = teaAnswer{status: "HTTP/1.1 404 Not Found", body: `{"message":"not found"}`}
	repoThere = teaAnswer{match: "/repos/forgejo/forgejo$", status: "HTTP/1.1 200 OK", body: `{"id": 73144}`}
	twoStuck  = `[
 {"number": 1, "state": "open", "draft": true, "mergeable": false, "head": {"sha": "a1", "repo": {"id": 7}}, "base": {"repo": {"id": 7}}},
 {"number": 2, "state": "open", "draft": false, "mergeable": false, "head": {"sha": "b2", "repo": {"id": 7}}, "base": {"repo": {"id": 7}}}
]`
)

func at(a teaAnswer, match string) teaAnswer { a.match = match; return a }

func teaWith(t *testing.T, first ...teaAnswer) (*forge.Router, string) {
	bin, calls := fakeTeaWith(t, codebergLogin, first, 0)
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://codeberg.org/forgejo/forgejo.git"}).url},
		Bins:    map[forge.Kind]string{forge.KindGitea: bin},
	}), calls
}

// A repository whose issues are off - a mirror, or one with an outside
// tracker - answers its issue list with 404. That is no issues here, not a
// project on no forge: its pull requests still read (#458).
func TestGitea_IssuesTurnedOffAreNone_issue458(t *testing.T) {
	r, _ := teaWith(t, at(notFound, "issues?state=open"), repoThere)

	if issues, err := r.ListIssues(t.TempDir()); err != nil || len(issues) != 0 {
		t.Errorf("ListIssues = %v, %v; want none and no error", issues, err)
	}
	if prs, err := r.ListPRs(t.TempDir()); err != nil || len(prs) == 0 {
		t.Errorf("ListPRs = %d, %v; want the pull requests read", len(prs), err)
	}
}

// Pull requests turned off are none to show, not a stopped project; a
// repository that is gone is still no forge.
func TestGitea_PullsTurnedOffAreNone_issue458(t *testing.T) {
	r, _ := teaWith(t, at(notFound, "pulls?state="), repoThere)
	if prs, err := r.ListPRs(t.TempDir()); err != nil || len(prs) != 0 {
		t.Errorf("ListPRs = %v, %v; want none and no error", prs, err)
	}
	gone, _ := teaWith(t, at(notFound, "pulls?state="), at(notFound, "/repos/forgejo/forgejo$"))
	if _, err := gone.ListPRs(t.TempDir()); !errors.Is(err, forge.ErrNoForge) {
		t.Errorf("a repository that is gone: %v, want ErrNoForge", err)
	}
}

// Gitea reports a draft as not mergeable, so a draft is not a conflict; an
// open one that cannot merge is (#458).
func TestGitea_ADraftIsNotAConflict_issue458(t *testing.T) {
	r, _ := teaWith(t, teaAnswer{"pulls?state=open", "HTTP/1.1 200 OK", twoStuck}, teaAnswer{"pulls?state=closed", "HTTP/1.1 200 OK", `[]`})

	prs, err := r.ListPRs(t.TempDir())

	if err != nil || len(prs) != 2 || prs[0].Conflict || !prs[1].Conflict {
		t.Errorf("ListPRs = %+v, %v; want the draft clear and the other a conflict", prs, err)
	}
}

// A commit with no statuses has no CI, whatever state Gitea puts beside it;
// and the combined status is asked for a whole page of checks, since Forgejo
// combines only the page it returns (#458).
func TestGitea_CIIsReadFromAWholePageOfChecks_issue458(t *testing.T) {
	r, calls := teaWith(t, teaAnswer{"/status", "HTTP/1.1 200 OK", `{"state": "pending", "total_count": 0, "statuses": []}`})

	prs, err := r.ListPRs(t.TempDir())

	if err != nil || len(prs) == 0 || prs[0].CI != forge.CINone {
		t.Errorf("ListPRs = %+v, %v; want no CI for a commit with no statuses", prs, err)
	}
	if b, _ := os.ReadFile(calls); !strings.Contains(string(b), "/status?limit=50") {
		t.Errorf("tea was called:\n%s\nwant the combined status asked fifty at a time", b)
	}
}

// Issues are asked as issues: Gitea's /issues lists pull requests too.
func TestGitea_IssuesAreAskedAsIssues_issue458(t *testing.T) {
	r, calls := teaWith(t)
	_, _ = r.ListIssues(t.TempDir())

	if b, _ := os.ReadFile(calls); !strings.Contains(string(b), "issues?state=open&type=issues") {
		t.Errorf("tea was called:\n%s\nwant type=issues", b)
	}
}

// tea exits 0 whatever the answer, so its status line is the verdict: 401 is
// its login refused, 5xx an outage - neither is a list of nothing (#458).
func TestGitea_TeasStatusLineIsTheVerdict_issue458(t *testing.T) {
	r, _ := teaWith(t, teaAnswer{"issues?state=open", "HTTP/1.1 401 Unauthorized", `{"message":"token is required"}`})
	var refused *forge.AuthError
	if _, err := r.ListIssues(t.TempDir()); !errors.As(err, &refused) || refused.Status != 401 {
		t.Errorf("401: %v, want tea's login refused", err)
	}
	r, _ = teaWith(t, teaAnswer{"issues?state=open", "HTTP/1.1 500 Internal Server Error", `{"message":"boom"}`})
	if _, err := r.ListIssues(t.TempDir()); err == nil || errors.As(err, &refused) {
		t.Errorf("500: %v, want an outage", err)
	}
}

// An answer's words are bounded in the error and the log: a proxy's error
// page can be kilobytes. A tea that printed no status line says so (#458).
func TestGitea_ATeaFailureIsBoundedAndSaysWhat_issue458(t *testing.T) {
	page := "<html>" + strings.Repeat("gateway ", 1000) + "</html>"
	r, _ := teaWith(t, teaAnswer{"issues?state=open", "HTTP/1.1 502 Bad Gateway", page})
	if _, err := r.ListIssues(t.TempDir()); err == nil || len(err.Error()) > 400 {
		t.Errorf("502: an error of %d bytes, want it bounded", len(err.Error()))
	}
	r, _ = teaWith(t, teaAnswer{"issues?state=open", "", `[]`})
	if _, err := r.ListIssues(t.TempDir()); err == nil || !strings.Contains(err.Error(), "no status line") {
		t.Errorf("no status line: %v, want it named", err)
	}
}

// A comment list that cannot be read leaves the item marked as not whole,
// never shown as an item nobody discussed (#397, #458).
func TestGitea_UnreadCommentsMarkTheItemCut_issue458(t *testing.T) {
	r, _ := teaWith(t, teaAnswer{"issues/2809/comments", "HTTP/1.1 500 Internal Server Error", `{}`})

	d, err := r.ViewIssue(t.TempDir(), 2809)

	if err != nil || !d.Truncated {
		t.Errorf("ViewIssue = truncated %v, %v; want the item marked cut", d.Truncated, err)
	}
}

// A tea older than 0.12 has no api command: it holds the login and can read
// nothing, so it is a missing tool, not "?" forever (#458).
func TestGitea_ATeaWithoutAPIIsTooOld_issue458(t *testing.T) {
	bin, calls := fakeTeaWith(t, codebergLogin, nil, 3)
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://codeberg.org/forgejo/forgejo.git"}).url},
		Bins:    map[forge.Kind]string{forge.KindGitea: bin},
	})
	_, _ = r.ListIssues(t.TempDir())
	_, err := r.ListIssues(t.TempDir())

	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "tea 0.12 or later" {
		t.Errorf("error = %v, want tea 0.12 or later missing", err)
	}
	if b, _ := os.ReadFile(calls); strings.Contains(string(b), "--login") || !strings.Contains(string(b), "api --help") {
		t.Errorf("tea was called:\n%s\nwant api --help asked and no read", b)
	}
}

// A tea login is found for the instance a remote names however the two
// write it: an ssh remote against a login on Gitea's own port, a login
// written in capitals, a login's separate ssh host, an https default port
// written out. A different port over https is a different instance (#458).
func TestGitea_ALoginIsFoundHoweverTheHostIsWritten_issue458(t *testing.T) {
	hosts := forge.Hosts{"git.example.com": forge.KindGitea, "ssh.example.com": forge.KindGitea}
	for _, c := range []struct {
		remote, loginURL, sshHost string
		found                     bool
	}{
		{"git@git.example.com:team/app.git", "https://git.example.com:3000", "", true},
		{"https://git.example.com/team/app.git", "https://Git.Example.com", "", true},
		{"git@ssh.example.com:team/app.git", "https://git.example.com", "ssh.example.com", true},
		{"https://git.example.com/team/app.git", "https://git.example.com:443", "", true},
		{"https://git.example.com:3000/team/app.git", "https://git.example.com", "", false},
	} {
		logins := `[{"name":"work","url":"` + c.loginURL + `","ssh_host":"` + c.sshHost + `"}]`
		bin, calls := fakeTeaWith(t, logins, nil, 0)
		r := forge.NewTestRouter(forge.TestEnv{
			Options: forge.Options{Remote: (&FakeRemote{URL: c.remote}).url, Hosts: hosts},
			Bins:    map[forge.Kind]string{forge.KindGitea: bin},
		})
		_, _ = r.ListIssues(t.TempDir())
		b, _ := os.ReadFile(calls)
		if used := strings.Contains(string(b), "--login work"); used != c.found {
			t.Errorf("%s against a login at %s (ssh %q): used = %v, want %v", c.remote, c.loginURL, c.sshHost, used, c.found)
		}
	}
}
