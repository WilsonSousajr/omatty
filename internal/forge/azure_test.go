package forge_test

import (
	"errors"
	"io"
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

// FakeAzureAPI is Azure DevOps's REST API as a test server over the fixtures
// in testdata/forge/azure/, keeping each request and any body it was sent.
type FakeAzureAPI struct {
	Status      int
	ContentType string
	mu          sync.Mutex
	Got         []FakeRequest
	Bodies      []string
}

var azureRoutes = []struct{ match, file string }{
	{"searchCriteria.status=active", "prs-active.json"},
	{"searchCriteria.status=completed", "prs-completed.json"},
	{"searchCriteria.status=abandoned", "prs-abandoned.json"},
	{"/policy/evaluations", "policy.json"},
	{"/pullRequests/11/threads", "threads.json"},
	{"/pullrequests/11?", "pr.json"},
	{"/wit/wiql", "wiql.json"},
	{"/wit/workitems?ids=", "workitems.json"},
	{"/workItems/101/comments", "comments.json"},
	{"/wit/workitems/101?", "workitem.json"},
}

func (f *FakeAzureAPI) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pq := r.URL.EscapedPath() + "?" + r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.Got = append(f.Got, FakeRequest{Host: r.Header.Get("X-Original-Host"), Path: pq, Auth: r.Header.Get("Authorization")})
		f.Bodies = append(f.Bodies, string(body))
		f.mu.Unlock()
		f.answer(t, w, pq)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (f *FakeAzureAPI) answer(t *testing.T, w http.ResponseWriter, pq string) {
	contentType := "application/json; charset=utf-8; api-version=7.1"
	if f.ContentType != "" {
		contentType = f.ContentType
	}
	w.Header().Set("Content-Type", contentType)
	if f.Status != 0 {
		w.WriteHeader(f.Status)
		return
	}
	for _, route := range azureRoutes {
		if strings.Contains(pq, route.match) {
			b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "forge", "azure", route.file))
			if err != nil {
				t.Error(err)
			}
			_, _ = w.Write(b)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

// fakeAz is an az whose `account get-access-token` prints a token for Azure
// DevOps's resource, as a logged-in az does.
func fakeAz(t *testing.T, token string) (bin, calls string) {
	t.Helper()
	return fakeGH(t, `{"accessToken": "`+token+`", "expiresOn": "2026-09-28 23:00:00.000000", "tokenType": "Bearer"}`, "", 0)
}

const azureRemote = "https://dev.azure.com/dnceng-public/public/_git/dotnet-public-wiki"

// azureRouter is a Router over the public wiki repository with az as given
// (empty: none), env as the environment, and every request sent to api.
func azureRouter(t *testing.T, az string, env map[string]string, api *FakeAzureAPI) *forge.Router {
	bins := map[forge.Kind]string{}
	if az != "" {
		bins[forge.KindAzure] = az
	}
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: azureRemote}).url},
		Bins:    bins, Env: env, API: api.serve(t),
	})
}

// Through the operator's az: the token az issues for Azure DevOps is sent as
// a bearer to the organisation's REST API (#456).
func TestAzure_ReadsWithAzsOwnLogin_issue456(t *testing.T) {
	az, calls := fakeAz(t, secret)
	api := &FakeAzureAPI{}

	if _, err := azureRouter(t, az, nil, api).ListPRs(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	got := api.Got[0]
	if got.Host != "dev.azure.com" || got.Auth != "Bearer "+secret ||
		!strings.HasPrefix(got.Path, "/dnceng-public/public/_apis/git/repositories/dotnet-public-wiki/pullrequests?") {
		t.Errorf("sent %+v, want a bearer to the repository's pull requests", got)
	}
	if b, _ := os.ReadFile(calls); !strings.Contains(string(b), "account get-access-token --resource 499b84ac-1321-427f-aa17-267ca6975798") {
		t.Errorf("az was asked %q, want a token for Azure DevOps", b)
	}
}

// Pull requests: head, branch, draft, conflict, fork, and CI from the build
// policies alone (a reviewer policy is not CI); completed ones merged.
func TestAzure_ListsPullRequests_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)

	prs, err := azureRouter(t, az, nil, &FakeAzureAPI{}).ListPRs(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}
	pr, _ := prNumbered(prs, 11)
	want := forge.PR{Number: 11, Title: "Retire the bootstrap script", Branch: "retire-bootstrap", State: forge.Open,
		CI: forge.CIFailing, Conflict: true, Head: "3aae318f1661c50c34effbbf6882119ed161f2d6", Updated: pr.Updated}
	if pr != want || pr.Updated.IsZero() {
		t.Errorf("!11 = %+v\nwant a conflicted pull request whose build policy failed", pr)
	}
	if draft, _ := prNumbered(prs, 12); !draft.Draft || !draft.Fork {
		t.Errorf("!12 = %+v, want a draft from a fork", draft)
	}
	if done, _ := prNumbered(prs, 5); done.State != forge.Merged || done.MergedAt.IsZero() {
		t.Errorf("!5 = %+v, want merged with its close time", done)
	}
}

// Work items are the issues: a WIQL query for the open ones, then their fields
// in one batch. It is a query, not a board.
func TestAzure_ListsWorkItemsAsIssues_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)
	api := &FakeAzureAPI{}

	issues, err := azureRouter(t, az, nil, api).ListIssues(t.TempDir())

	if err != nil || len(issues) != 2 || issues[0].Number != 101 || issues[0].Assignee != "Jane Doe" ||
		!reflect.DeepEqual(issues[0].Labels, []string{"bug", "wiki"}) || issues[0].URL != "https://dev.azure.com/dnceng-public/public/_workitems/edit/101" {
		t.Fatalf("issues = %+v, %v", issues, err)
	}
	wiql := strings.Join(api.Bodies, "\n")
	for _, want := range []string{"[System.TeamProject] = @project", "[System.State] NOT IN ('Closed','Done','Removed')"} {
		if !strings.Contains(wiql, want) {
			t.Errorf("the WIQL sent was %s\nwant it to hold %s", wiql, want)
		}
	}
}

// A work item in full: its description and comments as text, not HTML.
func TestAzure_ReadsAWorkItemAsText_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)

	d, err := azureRouter(t, az, nil, &FakeAzureAPI{}).ViewIssue(t.TempDir(), 101)

	if err != nil || d.Body != "Clone, then run build.cmd.\nIt fails at \"restore\"." || len(d.Comments) != 1 || d.Comments[0].Body != "Reproduced on main." {
		t.Errorf("detail = %+v, %v", d, err)
	}
}

// A pull request in full: its text comments, not the system ones or the
// deleted, and its build policies as checks.
func TestAzure_ReadsAPullRequestInFull_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)

	d, err := azureRouter(t, az, nil, &FakeAzureAPI{}).ViewPR(t.TempDir(), 11)

	if err != nil || d.Number != 11 || len(d.Comments) != 2 || len(d.Checks) != 1 || d.Checks[0].State != forge.CIFailing {
		t.Errorf("detail = %+v, %v; want two comments and one failing build", d, err)
	}
}

// Azure's words: pull requests written "!12" (#12 is a work item there), and
// its pages for browse.
func TestAzure_SpeaksAndBrowses_issue456(t *testing.T) {
	var opened []string
	az, _ := fakeAz(t, secret)
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: azureRemote}).url},
		Bins:    map[forge.Kind]string{forge.KindAzure: az}, API: (&FakeAzureAPI{}).serve(t),
		Open: func(u string) error { opened = append(opened, u); return nil },
	})
	root := t.TempDir()

	_ = r.BrowsePR(root, 11)
	_ = r.BrowseIssue(root, 101)

	if got := r.Label(root); got.Forge != "Azure DevOps" || got.Ref(12) != "!12" {
		t.Errorf("Label = %+v, want Azure's", got)
	}
	want := []string{"https://dev.azure.com/dnceng-public/public/_git/dotnet-public-wiki/pullrequest/11", "https://dev.azure.com/dnceng-public/public/_workitems/edit/101"}
	if !reflect.DeepEqual(opened, want) {
		t.Errorf("opened %v, want %v", opened, want)
	}
}

// Every Azure remote shape reaches the same organisation, project and
// repository: dev.azure.com with or without the org as a user, the ssh v3
// shape, and the old visualstudio.com host.
func TestAzure_EveryRemoteShapeReachesTheRepository_issue456(t *testing.T) {
	for url, want := range map[string]string{
		"https://org@dev.azure.com/org/My%20Project/_git/repo":             "/org/My%20Project/_apis/git/repositories/repo/",
		"git@ssh.dev.azure.com:v3/org/Project/repo":                        "/org/Project/_apis/git/repositories/repo/",
		"https://org.visualstudio.com/DefaultCollection/Project/_git/repo": "/DefaultCollection/Project/_apis/git/repositories/repo/",
		"https://dev.azure.com/org/Project/_git/_optimized/repo":           "/org/Project/_apis/git/repositories/repo/",
	} {
		az, _ := fakeAz(t, secret)
		api := &FakeAzureAPI{}
		r := forge.NewTestRouter(forge.TestEnv{
			Options: forge.Options{Remote: (&FakeRemote{URL: url}).url},
			Bins:    map[forge.Kind]string{forge.KindAzure: az}, API: api.serve(t),
		})

		_, _ = r.ListPRs(t.TempDir())

		if len(api.Got) == 0 || !strings.HasPrefix(api.Got[0].Path, want) {
			t.Errorf("%s: sent %+v, want %s...", url, api.Got, want)
		}
	}
}

// Without az there is nothing to ask until #457's PAT; the note names az.
func TestAzure_WithoutAzIsAMissingTool_issue456(t *testing.T) {
	_, err := azureRouter(t, "", nil, &FakeAzureAPI{}).ListPRs(t.TempDir())

	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "az" {
		t.Errorf("error = %v, want az missing", err)
	}
}
