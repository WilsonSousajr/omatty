package forge_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// FakeBitbucketDCAPI is a Bitbucket Data Center's REST API as a test server
// over the synthetic fixtures in testdata/forge/bitbucket-dc/.
type FakeBitbucketDCAPI struct {
	Status int
	mu     sync.Mutex
	Got    []FakeRequest
}

var bitbucketDCRoutes = []struct{ match, file string }{
	{"pull-requests?state=OPEN", "prs-open.json"},
	{"pull-requests?state=MERGED", "prs-merged.json"},
	{"pull-requests?state=DECLINED", "prs-declined.json"},
	{"/rest/build-status/1.0/commits/", "build-status.json"},
	{"pull-requests/42/activities", "activities.json"},
}

func (f *FakeBitbucketDCAPI) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pq := r.URL.Path + "?" + r.URL.RawQuery
		f.mu.Lock()
		f.Got = append(f.Got, FakeRequest{Host: r.Header.Get("X-Original-Host"), Path: pq, Auth: r.Header.Get("Authorization")})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		if f.Status != 0 {
			w.WriteHeader(f.Status)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/pull-requests/42") {
			serveDCPR42(t, w)
			return
		}
		for _, route := range bitbucketDCRoutes {
			if strings.Contains(pq, route.match) {
				b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "forge", "bitbucket-dc", route.file))
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

// serveDCPR42 answers pull request 42 alone, cut from the open list.
func serveDCPR42(t *testing.T, w http.ResponseWriter) {
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "forge", "bitbucket-dc", "prs-open.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, `{"id": 42`)
	end := strings.Index(s, "\n {\"id\": 41")
	_, _ = w.Write([]byte(strings.TrimSuffix(strings.TrimSpace(s[start:end]), ",")))
}

// dcRouter is a Router over OPS/platform on a Data Center host the operator
// named as Bitbucket, with the remote given.
func dcRouter(t *testing.T, url string, env map[string]string, api *FakeBitbucketDCAPI) *forge.Router {
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: url}).url, Hosts: forge.Hosts{"git.corp.example": forge.KindBitbucket}},
		Env:     env, API: api.serve(t),
	})
}

var dcToken = map[string]string{"BITBUCKET_TOKEN": secret}

// A Data Center host - any Bitbucket host but bitbucket.org - is read through
// its own /rest/api/1.0 with a bearer HTTP access token, and its clone URL's
// /scm/ prefix is not part of the repository (#461).
func TestBitbucketDC_ReadsItsOwnAPI_issue461(t *testing.T) {
	for _, url := range []string{"https://git.corp.example/scm/ops/platform.git", "ssh://git@git.corp.example:7999/ops/platform.git"} {
		api := &FakeBitbucketDCAPI{}

		if _, err := dcRouter(t, url, dcToken, api).ListPRs(t.TempDir()); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
		got := api.Got[0]
		if got.Host != "git.corp.example" || !strings.HasPrefix(got.Path, "/rest/api/1.0/projects/ops/repos/platform/pull-requests?") || got.Auth != "Bearer "+secret {
			t.Errorf("%s: sent %+v, want a bearer to /rest/api/1.0/projects/ops/repos/platform", url, got)
		}
	}
}

// Its pull requests: branch, head, draft, conflict from the merge result, fork
// from the project, CI from build-status, and the finished with their close.
func TestBitbucketDC_ListsPullRequests_issue461(t *testing.T) {
	prs, err := dcRouter(t, "https://git.corp.example/scm/ops/platform.git", dcToken, &FakeBitbucketDCAPI{}).ListPRs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	pr, _ := prNumbered(prs, 42)
	want := forge.PR{Number: 42, Title: "Add the audit export", Branch: "feature/audit-export", State: forge.Open,
		CI: forge.CIFailing, Conflict: true, Head: "8d51122def5632836d1cb1026e879069e10a1e13", Updated: pr.Updated}
	if pr != want || pr.Updated.IsZero() {
		t.Errorf("#42 = %+v\nwant %+v", pr, want)
	}
	if fork, _ := prNumbered(prs, 41); !fork.Fork || !fork.Draft {
		t.Errorf("#41 = %+v, want a draft from a personal fork", fork)
	}
	if merged, _ := prNumbered(prs, 40); merged.State != forge.Merged || merged.MergedAt.IsZero() {
		t.Errorf("#40 = %+v, want merged with its close time", merged)
	}
}

// A pull request in full: its description, its comments from its activity
// (not the approvals), its builds as checks.
func TestBitbucketDC_ReadsAPullRequestInFull_issue461(t *testing.T) {
	d, err := dcRouter(t, "https://git.corp.example/scm/ops/platform.git", dcToken, &FakeBitbucketDCAPI{}).ViewPR(t.TempDir(), 42)

	if err != nil || d.Body != "Exports the audit log as CSV." || d.Author != "Jane Doe" ||
		len(d.Comments) != 1 || d.Comments[0].Author != "Rick Roe" || len(d.Checks) != 2 {
		t.Errorf("detail = %+v, %v; want #42 with one comment and two builds", d, err)
	}
}

// Data Center has no issue tracker - the issues are in Jira.
func TestBitbucketDC_KeepsNoIssues_issue461(t *testing.T) {
	r := dcRouter(t, "https://git.corp.example/scm/ops/platform.git", dcToken, &FakeBitbucketDCAPI{})

	if _, err := r.ListIssues(t.TempDir()); !errors.Is(err, forge.ErrNoTracker) {
		t.Errorf("error = %v, want ErrNoTracker", err)
	}
}

// 401 is the token refused; a list's 404 is no forge; no token says so.
func TestBitbucketDC_ClassifiesTheAnswer_issue461(t *testing.T) {
	url := "https://git.corp.example/scm/ops/platform.git"
	_, err := dcRouter(t, url, dcToken, &FakeBitbucketDCAPI{Status: 401}).ListPRs(t.TempDir())
	var refused *forge.AuthError
	if !errors.As(err, &refused) || refused.TokenEnv != "BITBUCKET_TOKEN" {
		t.Errorf("401: error = %v, want BITBUCKET_TOKEN refused", err)
	}
	if _, err := dcRouter(t, url, dcToken, &FakeBitbucketDCAPI{Status: 404}).ListPRs(t.TempDir()); !errors.Is(err, forge.ErrNoForge) {
		t.Errorf("404: error = %v, want ErrNoForge", err)
	}
	var missing *forge.MissingToolError
	if _, err := dcRouter(t, url, nil, &FakeBitbucketDCAPI{}).ListPRs(t.TempDir()); !errors.As(err, &missing) {
		t.Errorf("no token: error = %v, want BITBUCKET_TOKEN unset", err)
	}
}

// b opens the pull request's page on the instance.
func TestBitbucketDC_BrowseOpensThePullRequest_issue461(t *testing.T) {
	var opened []string
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "ssh://git@git.corp.example:7999/ops/platform.git"}).url,
			Hosts: forge.Hosts{"git.corp.example": forge.KindBitbucket}},
		Env: dcToken, API: (&FakeBitbucketDCAPI{}).serve(t),
		Open: func(u string) error { opened = append(opened, u); return nil },
	})

	_ = r.BrowsePR(t.TempDir(), 42)

	if want := []string{"https://git.corp.example/projects/ops/repos/platform/pull-requests/42"}; !reflect.DeepEqual(opened, want) {
		t.Errorf("opened %v, want %v", opened, want)
	}
}

// An instance under a context path keeps it in every API and web URL, and a
// remote that is not <KEY>/<slug> is no forge rather than a guess.
func TestBitbucketDC_KeepsAContextPathAndRefusesOtherShapes_issue461(t *testing.T) {
	api := &FakeBitbucketDCAPI{}
	_, _ = dcRouter(t, "https://git.corp.example/bitbucket/scm/ops/platform.git", dcToken, api).ListPRs(t.TempDir())
	if len(api.Got) == 0 || !strings.HasPrefix(api.Got[0].Path, "/bitbucket/rest/api/1.0/projects/ops/repos/platform/") {
		t.Errorf("sent %+v, want the context path kept", api.Got)
	}
	_, err := dcRouter(t, "https://git.corp.example/scm/a/b/c.git", dcToken, &FakeBitbucketDCAPI{}).ListPRs(t.TempDir())
	if !errors.Is(err, forge.ErrNoForge) {
		t.Errorf("a three-part path: error = %v, want ErrNoForge", err)
	}
}
