package forge_test

import (
	"encoding/json"
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

// fixture reads one recorded answer from testdata/forge/<kind>/: public
// repositories only, never a token (AGENTS.md, Security).
func fixture(t *testing.T, kind, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "forge", kind, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// FakeGitHubAPI is GitHub's GraphQL endpoint as a test server, answering each
// query with its recorded fixture and keeping what it was sent.
type FakeGitHubAPI struct {
	PRs, Issues, Item string
	Status            int
	mu                sync.Mutex
	Got               []FakeRequest
}

// FakeRequest is one request as the fake received it.
type FakeRequest struct {
	Host, Path, Auth, Query string
}

func (f *FakeGitHubAPI) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal(body, &req)
		f.mu.Lock()
		f.Got = append(f.Got, FakeRequest{r.Header.Get("X-Original-Host"), r.URL.Path, r.Header.Get("Authorization"), req.Query})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if f.Status != 0 {
			w.WriteHeader(f.Status)
		}
		_, _ = w.Write([]byte(f.answer(req.Query)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (f *FakeGitHubAPI) answer(query string) string {
	switch {
	case strings.Contains(query, "issueOrPullRequest"):
		return f.Item
	case strings.Contains(query, "issues("):
		return f.Issues
	}
	return f.PRs
}

// recordedGitHub is the fake over the fixtures recorded from this repository.
func recordedGitHub(t *testing.T) *FakeGitHubAPI {
	return &FakeGitHubAPI{
		PRs:    fixture(t, "github", "graphql-prs.json"),
		Issues: fixture(t, "github", "graphql-issues.json"),
		Item:   fixture(t, "github", "graphql-item.json"),
	}
}

// httpRouter is a Router with no gh on PATH, the given environment, and every
// HTTP request sent to api.
func httpRouter(t *testing.T, url string, env map[string]string, api *FakeGitHubAPI, hosts forge.Hosts) *forge.Router {
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: url}).url, Hosts: hosts},
		Env:     env, API: api.serve(t).URL,
	})
}

// recordedGh is a gh answering with the same repository's recorded lists.
func recordedGh(t *testing.T) string {
	bin, _ := ghByState{Answers: map[string]string{
		"open": fixture(t, "github", "gh-open.json"), "closed": fixture(t, "github", "gh-closed.json"),
	}}.install(t)
	return bin
}

// The point of #462: over HTTP a card reads exactly what it reads over gh.
// Both fixtures were recorded from this repository within a minute.
func TestGitHubHTTP_ReadsTheSamePullRequestsAsGh_issue462(t *testing.T) {
	root := t.TempDir()
	viaGh, err := forge.NewCLIWithBin(recordedGh(t)).ListPRs(root)
	if err != nil {
		t.Fatal(err)
	}
	r := httpRouter(t, "git@github.com:WilsonSousajr/omatty.git", map[string]string{"GH_TOKEN": secret}, recordedGitHub(t), nil)

	viaHTTP, err := r.ListPRs(root)

	if err != nil || !reflect.DeepEqual(viaHTTP, viaGh) {
		t.Errorf("over HTTP: %v\n%+v\nover gh:\n%+v", err, viaHTTP, viaGh)
	}
}

// The same for the issues and for one item in full.
func TestGitHubHTTP_ReadsTheSameIssuesAndItemAsGh_issue462(t *testing.T) {
	issuesViaGh, _ := forge.FoldIssues([]byte(fixture(t, "github", "gh-issues.json")))
	itemViaGh, _ := forge.FoldDetail([]byte(fixture(t, "github", "gh-item.json")))
	r := httpRouter(t, "https://github.com/WilsonSousajr/omatty", map[string]string{"GH_TOKEN": secret}, recordedGitHub(t), nil)

	issues, err := r.ListIssues(t.TempDir())
	if err != nil || !reflect.DeepEqual(issues, issuesViaGh) {
		t.Errorf("issues over HTTP differ from gh's: %v", err)
	}
	item, err := r.ViewPR(t.TempDir(), 577)
	if err != nil || !reflect.DeepEqual(item, itemViaGh) {
		t.Errorf("item over HTTP: %v\n%+v\nover gh:\n%+v", err, item, itemViaGh)
	}
}

// The token goes to api.github.com's GraphQL as a bearer, from GH_TOKEN, and
// GITHUB_TOKEN stands in when GH_TOKEN is unset - gh's own order.
func TestGitHubHTTP_BorrowsGhsOwnTokenForGitHubsAPI_issue462(t *testing.T) {
	for _, env := range []map[string]string{{"GH_TOKEN": secret}, {"GITHUB_TOKEN": secret}} {
		api := recordedGitHub(t)
		r := httpRouter(t, "git@github.com:WilsonSousajr/omatty.git", env, api, nil)

		if _, err := r.ListIssues(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		if got := api.Got[0]; got.Host != "api.github.com" || got.Path != "/graphql" || got.Auth != "Bearer "+secret {
			t.Errorf("env %v: sent %+v, want a bearer to api.github.com/graphql", env, got)
		}
	}
}

// An Enterprise host is asked at its own /api/graphql, with the Enterprise
// token gh reads there.
func TestGitHubHTTP_AnEnterpriseHostIsAskedAtItsOwnAPI_issue462(t *testing.T) {
	api := recordedGitHub(t)
	r := httpRouter(t, "https://ghe.corp.example/WilsonSousajr/omatty.git",
		map[string]string{"GH_TOKEN": "not-this-one", "GH_ENTERPRISE_TOKEN": secret}, api,
		forge.Hosts{"ghe.corp.example": forge.KindGitHub})

	if _, err := r.ListPRs(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if got := api.Got[0]; got.Host != "ghe.corp.example" || got.Path != "/api/graphql" || got.Auth != "Bearer "+secret {
		t.Errorf("sent %+v, want GH_ENTERPRISE_TOKEN to ghe.corp.example/api/graphql", got)
	}
}

// With neither gh nor a token the note names both halves of the fix.
func TestRouter_WithoutGhOrATokenNamesBoth_issue462(t *testing.T) {
	r := httpRouter(t, "git@github.com:o/r.git", nil, recordedGitHub(t), nil)

	_, err := r.ListPRs(t.TempDir())

	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "gh" || missing.TokenEnv != "GH_TOKEN" {
		t.Errorf("error = %v, want gh missing and GH_TOKEN unset", err)
	}
}

// CLI first: with gh on PATH a token in the environment is not used.
func TestRouter_GhWinsOverAToken_issue462(t *testing.T) {
	api := recordedGitHub(t)
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@github.com:o/r.git"}).url},
		GH:      recordedGh(t), Env: map[string]string{"GH_TOKEN": secret}, API: api.serve(t).URL,
	})

	if _, err := r.ListPRs(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if len(api.Got) != 0 {
		t.Errorf("the API was asked %d times with gh installed", len(api.Got))
	}
}

// forgeprobe's -transport rest reads over HTTP though gh is installed.
func TestRouter_ARESTTransportSkipsGh_issue462(t *testing.T) {
	api := recordedGitHub(t)
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@github.com:o/r.git"}).url, Transport: forge.TransportREST},
		GH:      recordedGh(t), Env: map[string]string{"GH_TOKEN": secret}, API: api.serve(t).URL,
	})

	if _, err := r.ListPRs(t.TempDir()); err != nil || len(api.Got) != 1 {
		t.Errorf("ListPRs = %v after %d API calls, want one", err, len(api.Got))
	}
}

// A repository the token cannot see answers NOT_FOUND inside a 200, and that is
// a project with no forge to read, as a 404 is.
func TestGitHubHTTP_ARepositoryItCannotSeeIsErrNoForge_issue462(t *testing.T) {
	api := &FakeGitHubAPI{PRs: `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","message":"Could not resolve to a Repository"}]}`}
	r := httpRouter(t, "git@github.com:o/private.git", map[string]string{"GH_TOKEN": secret}, api, nil)

	if _, err := r.ListPRs(t.TempDir()); !errors.Is(err, forge.ErrNoForge) {
		t.Errorf("error = %v, want ErrNoForge", err)
	}
}

// b opens the item's own page: /issues/N or /pull/N on the web host.
func TestGitHubHTTP_BrowseOpensTheItemsPage_issue462(t *testing.T) {
	var opened []string
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@github.com:WilsonSousajr/omatty.git"}).url},
		Env:     map[string]string{"GH_TOKEN": secret}, API: recordedGitHub(t).serve(t).URL,
		Open: func(url string) error { opened = append(opened, url); return nil },
	})

	_ = r.BrowseIssue(t.TempDir(), 399)
	_ = r.BrowsePR(t.TempDir(), 400)

	want := []string{"https://github.com/WilsonSousajr/omatty/issues/399", "https://github.com/WilsonSousajr/omatty/pull/400"}
	if !reflect.DeepEqual(opened, want) {
		t.Errorf("opened %v, want %v", opened, want)
	}
}

// Shipping over HTTP is #464's; until then it refuses, and a protection flag it
// cannot read is protected.
func TestGitHubHTTP_ShippingRefusesUntilItIsBuilt_issue462(t *testing.T) {
	r := httpRouter(t, "git@github.com:o/r.git", map[string]string{"GH_TOKEN": secret}, recordedGitHub(t), nil)

	if _, err := r.CreatePR(t.TempDir(), "feat/a", "develop", "t"); err == nil {
		t.Error("CreatePR over HTTP succeeded before #464")
	}
	if protected, err := r.BranchProtected(t.TempDir(), "main"); !protected || err == nil {
		t.Errorf("BranchProtected = %v, %v; want true beside an error", protected, err)
	}
}
