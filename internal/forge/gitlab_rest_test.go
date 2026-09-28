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

// FakeGitLabAPI is GitLab's REST API as a test server over the same recorded
// fixtures the fake glab answers from, keeping each request it was sent.
type FakeGitLabAPI struct {
	Status int // when set, every answer is this status and no body
	mu     sync.Mutex
	Got    []FakeRequest
}

// gitLabRoutes maps a request's escaped path and query to its fixture, the
// same table fakeGlab's case statement is.
var gitLabRoutes = []struct{ match, file string }{
	{"merge_requests?state=opened", "mrs-opened.json"},
	{"merge_requests?state=merged", "mrs-merged.json"},
	{"merge_requests?state=closed", "mrs-closed.json"},
	{"/pipelines?per_page=1", "mr-pipelines.json"},
	{"merge_requests/3967/notes", "mr-notes.json"},
	{"/jobs", "jobs.json"},
	{"issues?state=opened", "issues.json"},
	{"issues/8566/notes", "issue-notes.json"},
	{"merge_requests/3967", "mr.json"},
	{"merge_requests/3966", "mr-3966.json"},
	{"merge_requests/3963", "mr-3963.json"},
	{"issues/8566", "issue.json"},
}

func (f *FakeGitLabAPI) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.Got = append(f.Got, FakeRequest{Host: r.Header.Get("X-Original-Host"), Path: r.URL.EscapedPath(), Auth: r.Header.Get("PRIVATE-TOKEN")})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if f.Status != 0 {
			w.WriteHeader(f.Status)
			return
		}
		serveFixture(t, w, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func serveFixture(t *testing.T, w http.ResponseWriter, pathAndQuery string) {
	for _, route := range gitLabRoutes {
		if strings.Contains(pathAndQuery, route.match) {
			b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "forge", "gitlab", route.file))
			if err != nil {
				t.Error(err)
			}
			_, _ = w.Write(b)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

// restGitLab is a Router with no glab, GITLAB_TOKEN set, and every HTTP
// request sent to api.
func restGitLab(t *testing.T, url string, api *FakeGitLabAPI, hosts forge.Hosts) *forge.Router {
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: url}).url, Hosts: hosts},
		Env:     map[string]string{"GITLAB_TOKEN": secret}, API: api.serve(t),
	})
}

// #455's point: over REST a card reads exactly what it reads through glab -
// one fixture, one fold, two transports.
func TestGitLabREST_ReadsWhatGlabReads_issue455(t *testing.T) {
	root := t.TempDir()
	glab, _ := gitLabRouter(t, "git@gitlab.com:gitlab-org/cli.git", nil)
	rest := restGitLab(t, "git@gitlab.com:gitlab-org/cli.git", &FakeGitLabAPI{}, nil)

	for name, read := range map[string]func(*forge.Router) (any, error){
		"ListPRs":    func(r *forge.Router) (any, error) { return r.ListPRs(root) },
		"ListIssues": func(r *forge.Router) (any, error) { return r.ListIssues(root) },
		"ViewPR":     func(r *forge.Router) (any, error) { return r.ViewPR(root, 3967) },
		"ViewIssue":  func(r *forge.Router) (any, error) { return r.ViewIssue(root, 8566) },
	} {
		viaGlab, err1 := read(glab)
		viaREST, err2 := read(rest)
		if err1 != nil || err2 != nil || !reflect.DeepEqual(viaREST, viaGlab) {
			t.Errorf("%s: over REST (%v)\n%+v\nthrough glab (%v)\n%+v", name, err2, viaREST, err1, viaGlab)
		}
	}
}

// The token rides in GitLab's own header to the project's host, and the
// project is addressed by its whole encoded path.
func TestGitLabREST_SendsGitLabsHeaderToTheProjectsHost_issue455(t *testing.T) {
	api := &FakeGitLabAPI{}
	r := restGitLab(t, "git@gitlab.com:gitlab-org/cli.git", api, nil)

	if _, err := r.ListIssues(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	got := api.Got[0]
	if got.Host != "gitlab.com" || got.Path != "/api/v4/projects/gitlab-org%2Fcli/issues" || got.Auth != secret {
		t.Errorf("sent %+v, want PRIVATE-TOKEN to gitlab.com/api/v4/projects/gitlab-org%%2Fcli/issues", got)
	}
}

// A self-managed instance on its own https port is asked there.
func TestGitLabREST_ASelfManagedPortIsKept_issue455(t *testing.T) {
	api := &FakeGitLabAPI{}
	r := restGitLab(t, "https://git.corp.example:8443/group/sub/project.git", api, forge.Hosts{"git.corp.example": forge.KindGitLab})

	_, _ = r.ListIssues(t.TempDir())

	if got := api.Got[0]; got.Host != "git.corp.example:8443" || !strings.HasPrefix(got.Path, "/api/v4/projects/group%2Fsub%2Fproject/") {
		t.Errorf("sent %+v, want git.corp.example:8443 and the whole project path", got)
	}
}

// 401 is the token refused - a note naming GITLAB_TOKEN - and 404 a project the
// token cannot see.
func TestGitLabREST_ClassifiesTheAnswer_issue455(t *testing.T) {
	_, err := restGitLab(t, "git@gitlab.com:g/p.git", &FakeGitLabAPI{Status: 401}, nil).ListIssues(t.TempDir())
	var refused *forge.AuthError
	if !errors.As(err, &refused) || refused.TokenEnv != "GITLAB_TOKEN" {
		t.Errorf("401: error = %v, want GITLAB_TOKEN refused", err)
	}
	if _, err := restGitLab(t, "git@gitlab.com:g/p.git", &FakeGitLabAPI{Status: 404}, nil).ListIssues(t.TempDir()); !errors.Is(err, forge.ErrNoForge) {
		t.Errorf("404: error = %v, want ErrNoForge", err)
	}
}

// With neither glab nor GITLAB_TOKEN the note names both halves of the fix.
func TestGitLabREST_WithNeitherNamesBoth_issue455(t *testing.T) {
	r := forge.NewTestRouter(forge.TestEnv{Options: forge.Options{Remote: (&FakeRemote{URL: "git@gitlab.com:g/p.git"}).url}})

	_, err := r.ListPRs(t.TempDir())

	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "glab" || missing.TokenEnv != "GITLAB_TOKEN" {
		t.Errorf("error = %v, want glab missing and GITLAB_TOKEN unset", err)
	}
}
