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
		f.Got = append(f.Got, FakeRequest{
			Host: r.Header.Get("X-Original-Host"), Path: r.URL.EscapedPath(), Query: r.URL.RawQuery, Auth: r.Header.Get("PRIVATE-TOKEN"),
		})
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
			b, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "forge", "gitlab", route.file))
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
	var refused *dforge.AuthError
	if !errors.As(err, &refused) || refused.TokenEnv != "GITLAB_TOKEN" {
		t.Errorf("401: error = %v, want GITLAB_TOKEN refused", err)
	}
	if _, err := restGitLab(t, "git@gitlab.com:g/p.git", &FakeGitLabAPI{Status: 404}, nil).ListIssues(t.TempDir()); !errors.Is(err, dforge.ErrNoForge) {
		t.Errorf("404: error = %v, want ErrNoForge", err)
	}
}

// With neither glab nor GITLAB_TOKEN the note names both halves of the fix.
func TestGitLabREST_WithNeitherNamesBoth_issue455(t *testing.T) {
	r := forge.NewTestRouter(forge.TestEnv{Options: forge.Options{Remote: (&FakeRemote{URL: "git@gitlab.com:g/p.git"}).url}})

	_, err := r.ListPRs(t.TempDir())

	var missing *dforge.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "glab" || missing.TokenEnv != "GITLAB_TOKEN" {
		t.Errorf("error = %v, want glab missing and GITLAB_TOKEN unset", err)
	}
}

// An http remote is one only glab can read: the token never crosses plain
// http, so with no glab the project stops with a note rather than asking
// every poll for an answer that cannot come. A port-less one included - it
// was once asked over https instead, a dial error on an http-only host.
func TestGitLabREST_AnHTTPRemoteStopsWithANote_issue584(t *testing.T) {
	hosts := forge.Hosts{"git.corp.example": forge.KindGitLab}
	for _, url := range []string{"http://git.corp.example:8080/g/p.git", "http://git.corp.example/g/p.git"} {
		api := &FakeGitLabAPI{}
		_, err := restGitLab(t, url, api, hosts).ListIssues(t.TempDir())

		var missing *dforge.MissingToolError
		if !errors.As(err, &missing) || missing.Tool != "glab" || missing.TokenEnv != "" {
			t.Errorf("%s: error = %v, want glab missing, with no token that could help", url, err)
		}
		if len(api.Got) != 0 {
			t.Errorf("%s: asked %+v, want nothing sent", url, api.Got)
		}
	}
}

// GITLAB_ACCESS_TOKEN is glab's other name for the same token.
func TestGitLabREST_BorrowsGlabsOtherName_issue455(t *testing.T) {
	api := &FakeGitLabAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@gitlab.com:gitlab-org/cli.git"}).url},
		Env:     map[string]string{"GITLAB_ACCESS_TOKEN": secret}, API: api.serve(t),
	})

	if _, err := r.ListIssues(t.TempDir()); err != nil || api.Got[0].Auth != secret {
		t.Errorf("ListIssues = %v, sent %+v; want GITLAB_ACCESS_TOKEN in PRIVATE-TOKEN", err, api.Got)
	}
}

// forgeprobe's transports: REST forced reads REST with glab installed, and
// CLI forced never falls back to REST with the token set (#463).
func TestGitLabREST_ATransportByNameIsHonoured_issue455(t *testing.T) {
	glab, calls := fakeGlab(t)
	env := func(tr forge.Transport, api *FakeGitLabAPI, bins map[forge.Kind]string) forge.TestEnv {
		return forge.TestEnv{
			Options: forge.Options{Remote: (&FakeRemote{URL: "git@gitlab.com:gitlab-org/cli.git"}).url, Transport: tr},
			Bins:    bins, Env: map[string]string{"GITLAB_TOKEN": secret}, API: api.serve(t),
		}
	}
	api := &FakeGitLabAPI{}
	withGlab := map[forge.Kind]string{forge.KindGitLab: glab}
	if _, err := forge.NewTestRouter(env(forge.TransportREST, api, withGlab)).ListIssues(t.TempDir()); err != nil || len(api.Got) == 0 {
		t.Errorf("REST forced: %v, %d requests; want REST read with glab installed", err, len(api.Got))
	}
	if b, _ := os.ReadFile(calls); len(b) != 0 {
		t.Errorf("REST forced ran glab: %s", b)
	}
	var missing *dforge.MissingToolError
	if _, err := forge.NewTestRouter(env(forge.TransportCLI, &FakeGitLabAPI{}, nil)).ListIssues(t.TempDir()); !errors.As(err, &missing) {
		t.Errorf("CLI forced with no glab: %v, want glab missing though GITLAB_TOKEN is set", err)
	}
}

// Every list asks as many as the card and tracker show, in one page.
func TestGitLabREST_ListsAskAWholePage_issue455(t *testing.T) {
	api := &FakeGitLabAPI{}
	r := restGitLab(t, "git@gitlab.com:gitlab-org/cli.git", api, nil)
	_, _ = r.ListPRs(t.TempDir())
	_, _ = r.ListIssues(t.TempDir())

	for _, want := range []string{
		"/merge_requests?state=opened&per_page=100", "/merge_requests?state=merged&per_page=30",
		"/merge_requests?state=closed&per_page=30", "/issues?state=opened&per_page=100",
	} {
		if !asked(api.Got, want) {
			t.Errorf("no request asked %q in %+v", want, api.Got)
		}
	}
}

func asked(got []FakeRequest, pathAndQuery string) bool {
	for _, g := range got {
		if strings.Contains(g.Path+"?"+g.Query, pathAndQuery) {
			return true
		}
	}
	return false
}
