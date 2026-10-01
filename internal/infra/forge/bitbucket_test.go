package forge_test

import (
	"errors"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/forge"
)

// FakeBitbucketAPI is Bitbucket Cloud's REST API (/2.0) as a test server over
// fixtures recorded from bitbucket.org/atlassianlabs/atlascode.
type FakeBitbucketAPI struct {
	Status int
	First  []giteaAnswer // answers tried before the fixtures
	mu     sync.Mutex
	Got    []FakeRequest
}

var bitbucketRoutes = []struct{ match, file string }{
	{"pullrequests?state=OPEN", "prs-open.json"},
	{"pullrequests?state=MERGED", "prs-finished.json"},
	// Each head's own statuses, so statuses read for the wrong commit are a 404.
	{"/commit/31b8ff8dad0a/statuses", "statuses.json"},
	{"/commit/c5e44c05d186/statuses", "statuses.json"},
	{"pullrequests/1115/comments", "pr-comments.json"},
	{"pullrequests/1115", "pr.json"},
}

func (f *FakeBitbucketAPI) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pq := r.URL.Path + "?" + r.URL.RawQuery
		f.mu.Lock()
		f.Got = append(f.Got, FakeRequest{Host: r.Header.Get("X-Original-Host"), Path: pq, Auth: r.Header.Get("Authorization")})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if f.Status != 0 {
			w.WriteHeader(f.Status)
			return
		}
		if scriptedAnswer(w, pq, f.First) {
			return
		}
		for _, route := range bitbucketRoutes {
			if strings.Contains(pq, route.match) {
				b, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "forge", "bitbucket", route.file))
				if err != nil {
					t.Error(err)
				}
				_, _ = w.Write(b)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// bitbucketRouter is a Router over atlascode on bitbucket.org with env as the
// environment and every request sent to api.
func bitbucketRouter(t *testing.T, env map[string]string, api *FakeBitbucketAPI) *forge.Router {
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@bitbucket.org:atlassianlabs/atlascode.git"}).url},
		Env:     env, API: api.serve(t),
	})
}

var bitbucketToken = map[string]string{"BITBUCKET_USER": "someone", "BITBUCKET_TOKEN": secret}

// A Bitbucket Cloud project's pull requests reach the card with branch, head,
// draft, fork and CI from their commit's statuses; finished ones are merged
// or declined (#460).
func TestBitbucket_ListsPullRequestsWithTheirCI_issue460(t *testing.T) {
	prs, err := bitbucketRouter(t, bitbucketToken, &FakeBitbucketAPI{}).ListPRs(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}
	pr, _ := prNumbered(prs, 1115)
	want := dforge.PR{Number: 1115, Title: pr.Title, Branch: pr.Branch, Base: "main", State: dforge.Open, CI: dforge.CIPassing, Head: "31b8ff8dad0a", Updated: pr.Updated}
	if pr != want || !strings.HasPrefix(pr.Branch, "feature/VULN-1872862") || pr.Updated.IsZero() {
		t.Errorf("#1115 = %+v, want open on its feature branch with a passing pipeline", pr)
	}
}

// The recently finished come too: merged, with the time Bitbucket last
// touched them, and declined as closed.
func TestBitbucket_ListsTheRecentlyFinished_issue460(t *testing.T) {
	prs, err := bitbucketRouter(t, bitbucketToken, &FakeBitbucketAPI{}).ListPRs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	states := map[dforge.PRState]int{}
	for _, pr := range prs {
		states[pr.State]++
	}
	if states[dforge.Merged] != 1 || states[dforge.Closed] != 2 {
		t.Errorf("finished states = %v, want 1 merged and 2 declined", states)
	}
}

// A commit status maps to the card's CI mark; unknown is running.
func TestBitbucket_StatusIsTheCIMark_issue460(t *testing.T) {
	for status, want := range map[string]dforge.CIState{
		"SUCCESSFUL": dforge.CIPassing, "FAILED": dforge.CIFailing, "STOPPED": dforge.CIFailing,
		"INPROGRESS": dforge.CIRunning, "SOMETHING_NEW": dforge.CIRunning,
	} {
		if got := forge.BitbucketCI(status); got != want {
			t.Errorf("status %q = %v, want %v", status, got, want)
		}
	}
}

// A pull request in full: its description, its comments, its statuses.
func TestBitbucket_ReadsAPullRequestInFull_issue460(t *testing.T) {
	d, err := bitbucketRouter(t, bitbucketToken, &FakeBitbucketAPI{}).ViewPR(t.TempDir(), 1115)

	if err != nil || d.Number != 1115 || d.Author != "Ray Zhang" || len(d.Body) < 100 ||
		len(d.Comments) != 2 || d.Comments[0].Author != "Rovo Dev" || len(d.Checks) != 1 ||
		d.URL != "https://bitbucket.org/atlassianlabs/atlascode/pull-requests/1115" {
		t.Errorf("detail = %+v, %v; want #1115 with its body, two comments and one check", d, err)
	}
}

// Bitbucket Cloud retired its issue tracker API (CHANGE-3071, 410 Gone on
// every repository as of 2026-09-28), so a project's issues live elsewhere,
// usually Jira: the tracker shows its pull requests and says so.
func TestBitbucket_IssuesLiveElsewhere_issue460(t *testing.T) {
	r := bitbucketRouter(t, bitbucketToken, &FakeBitbucketAPI{})

	if _, err := r.ListIssues(t.TempDir()); !errors.Is(err, dforge.ErrNoTracker) {
		t.Errorf("ListIssues error = %v, want ErrNoTracker", err)
	}
	if _, err := r.ViewIssue(t.TempDir(), 1); !errors.Is(err, dforge.ErrNoTracker) {
		t.Errorf("ViewIssue error = %v, want ErrNoTracker", err)
	}
}

// An API token with its user is Basic auth; a repository or workspace access
// token alone is a bearer. Neither set says which to set.
func TestBitbucket_BorrowsItsToken_issue460(t *testing.T) {
	for _, tt := range []struct {
		env  map[string]string
		want string
	}{
		{bitbucketToken, "Basic "},
		{map[string]string{"BITBUCKET_TOKEN": secret}, "Bearer " + secret},
	} {
		api := &FakeBitbucketAPI{}
		if _, err := bitbucketRouter(t, tt.env, api).ListPRs(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		if got := api.Got[0]; got.Host != "api.bitbucket.org" || !strings.HasPrefix(got.Path, "/2.0/repositories/atlassianlabs/atlascode/pullrequests?") || !strings.HasPrefix(got.Auth, tt.want) {
			t.Errorf("env %v: sent %+v, want %q... to api.bitbucket.org/2.0", tt.env, got, tt.want)
		}
	}
	_, err := bitbucketRouter(t, nil, &FakeBitbucketAPI{}).ListPRs(t.TempDir())
	var missing *dforge.MissingToolError
	if !errors.As(err, &missing) || missing.TokenEnv != "BITBUCKET_TOKEN" {
		t.Errorf("no token: error = %v, want BITBUCKET_TOKEN unset", err)
	}
}

// 401 is the token refused; a list's 404 is no forge.
func TestBitbucket_ClassifiesTheAnswer_issue460(t *testing.T) {
	_, err := bitbucketRouter(t, bitbucketToken, &FakeBitbucketAPI{Status: 401}).ListPRs(t.TempDir())
	var refused *dforge.AuthError
	if !errors.As(err, &refused) || refused.TokenEnv != "BITBUCKET_TOKEN" {
		t.Errorf("401: error = %v, want BITBUCKET_TOKEN refused", err)
	}
	// A 404 was ErrNoForge here until #460's review: with a token on every
	// read, Bitbucket's 404 is a token that cannot see the repository, and
	// TestBitbucket_A404WithATokenNamesTheToken_issue460 holds it.
}

// Bitbucket's words: pull requests, "#12". b opens the pull request's page.
func TestBitbucket_SpeaksAndBrowses_issue460(t *testing.T) {
	var opened []string
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@bitbucket.org:atlassianlabs/atlascode.git"}).url},
		Env:     bitbucketToken, API: (&FakeBitbucketAPI{}).serve(t),
		Open: func(u string) error { opened = append(opened, u); return nil },
	})
	root := t.TempDir()

	_ = r.BrowsePR(root, 1115)

	if got := r.Label(root); got.Forge != "Bitbucket" || got.Ref(12) != "#12" {
		t.Errorf("Label = %+v, want Bitbucket's", got)
	}
	if want := []string{"https://bitbucket.org/atlassianlabs/atlascode/pull-requests/1115"}; !reflect.DeepEqual(opened, want) {
		t.Errorf("opened %v, want %v", opened, want)
	}
}
