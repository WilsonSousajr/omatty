package forge_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// FakeGiteaAPI is Gitea's REST API as a test server over the recorded
// fixtures the fake tea answers from, keeping each request it was sent.
type FakeGiteaAPI struct {
	Status int           // when set, every answer is this status and no body
	Full   bool          // when set, open pull requests come fifty to a page, forever
	First  []giteaAnswer // answers tried before the fixtures
	mu     sync.Mutex
	Got    []FakeRequest
}

func (f *FakeGiteaAPI) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.Got = append(f.Got, FakeRequest{
			Host: r.Header.Get("X-Original-Host"), Path: r.URL.Path + "?" + r.URL.RawQuery, Auth: r.Header.Get("Authorization"),
			Scheme: r.Header.Get("X-Original-Scheme"),
		})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		f.answer(t, w, r.URL.Path+"?"+r.URL.RawQuery)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// giteaAnswer is one scripted answer for a path and query holding match.
type giteaAnswer struct {
	match, contentType, body string
	status                   int
}

func (f *FakeGiteaAPI) answer(t *testing.T, w http.ResponseWriter, pathAndQuery string) {
	for _, a := range f.First {
		if strings.Contains(pathAndQuery, a.match) {
			if a.contentType != "" {
				w.Header().Set("Content-Type", a.contentType)
			}
			w.WriteHeader(a.status)
			_, _ = w.Write([]byte(a.body))
			return
		}
	}
	if f.Status != 0 {
		w.WriteHeader(f.Status)
		return
	}
	if f.Full && strings.Contains(pathAndQuery, "pulls?state=open") {
		fiftyPulls(t, w)
		return
	}
	for _, r := range giteaRoutes {
		if strings.Contains(pathAndQuery, r.match) {
			b, err := os.ReadFile(giteaFixture(t, r.file))
			if err != nil {
				t.Error(err)
			}
			_, _ = w.Write(b)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

// fiftyPulls is a full page: fifty copies of the first recorded pull request,
// numbered apart.
func fiftyPulls(t *testing.T, w http.ResponseWriter) {
	b, err := os.ReadFile(giteaFixture(t, "pulls-open.json"))
	if err != nil {
		t.Fatal(err)
	}
	var one []map[string]json.RawMessage
	if err := json.Unmarshal(b, &one); err != nil {
		t.Fatal(err)
	}
	page := make([]map[string]json.RawMessage, 50)
	for i := range page {
		pr := map[string]json.RawMessage{}
		for k, v := range one[0] {
			pr[k] = v
		}
		pr["number"] = json.RawMessage(strconv.Itoa(i + 1))
		page[i] = pr
	}
	_ = json.NewEncoder(w).Encode(page)
}

// restGitea is a Router over forgejo/forgejo on Codeberg with no tea, env as
// its environment, and every HTTP request sent to api.
func restGitea(t *testing.T, env map[string]string, api *FakeGiteaAPI) *forge.Router {
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://codeberg.org/forgejo/forgejo.git"}).url},
		Env:     env, API: api.serve(t),
	})
}

// #459's point: over REST a card reads exactly what it reads through tea.
func TestGiteaREST_ReadsWhatTeaReads_issue459(t *testing.T) {
	root := t.TempDir()
	tea, _ := teaRouter(t, codebergLogin)
	rest := restGitea(t, nil, &FakeGiteaAPI{})

	for name, read := range map[string]func(*forge.Router) (any, error){
		"ListPRs":    func(r *forge.Router) (any, error) { return r.ListPRs(root) },
		"ListIssues": func(r *forge.Router) (any, error) { return r.ListIssues(root) },
		"ViewPR":     func(r *forge.Router) (any, error) { return r.ViewPR(root, 14587) },
		"ViewIssue":  func(r *forge.Router) (any, error) { return r.ViewIssue(root, 2809) },
	} {
		viaTea, err1 := read(tea)
		viaREST, err2 := read(rest)
		if err1 != nil || err2 != nil || !reflect.DeepEqual(viaREST, viaTea) {
			t.Errorf("%s: over REST (%v)\n%+v\nthrough tea (%v)\n%+v", name, err2, viaREST, err1, viaTea)
		}
	}
}

// Codeberg's public repositories are read with no token and no Authorization
// header at all.
func TestGiteaREST_ReadsAPublicRepositoryAnonymously_issue459(t *testing.T) {
	api := &FakeGiteaAPI{}

	if _, err := restGitea(t, nil, api).ListIssues(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	got := api.Got[0]
	if got.Host != "codeberg.org" || !strings.HasPrefix(got.Path, "/api/v1/repos/forgejo/forgejo/issues?") || got.Auth != "" {
		t.Errorf("sent %+v, want an anonymous read of codeberg.org/api/v1/repos/forgejo/forgejo/issues", got)
	}
}

// codebergToken is tea's env login for Codeberg: the token and the instance
// it is for.
var codebergToken = map[string]string{"GITEA_TOKEN": secret, "GITEA_INSTANCE_URL": "https://codeberg.org"}

// With GITEA_TOKEN set for this instance it is sent the way Gitea reads it.
func TestGiteaREST_SendsTheTokenAsGiteaReadsIt_issue459(t *testing.T) {
	api := &FakeGiteaAPI{}

	_, _ = restGitea(t, codebergToken, api).ListIssues(t.TempDir())

	if got := api.Got[0].Auth; got != "token "+secret {
		t.Errorf("Authorization = %q, want the token scheme", got)
	}
}

// Read anonymously, a refusal - or a 404, which is how Gitea answers a
// stranger about a private repository - says to set GITEA_TOKEN, rather than
// going quiet as a project on no forge.
func TestGiteaREST_AnAnonymousRefusalAsksForAToken_issue459(t *testing.T) {
	for _, status := range []int{401, 403, 404} {
		_, err := restGitea(t, nil, &FakeGiteaAPI{Status: status}).ListPRs(t.TempDir())

		var missing *forge.MissingToolError
		if !errors.As(err, &missing) || !strings.HasPrefix(missing.TokenEnv, "GITEA_TOKEN") {
			t.Errorf("%d: error = %v, want a note naming GITEA_TOKEN", status, err)
		}
	}
}

// With a token, a refusal is the token refused, and a 404 is no forge.
func TestGiteaREST_WithATokenTheUsualRulesHold_issue459(t *testing.T) {
	env := codebergToken
	_, err := restGitea(t, env, &FakeGiteaAPI{Status: 401}).ListIssues(t.TempDir())
	var refused *forge.AuthError
	if !errors.As(err, &refused) || refused.TokenEnv != "GITEA_TOKEN" {
		t.Errorf("401: error = %v, want GITEA_TOKEN refused", err)
	}
	if _, err := restGitea(t, env, &FakeGiteaAPI{Status: 404}).ListIssues(t.TempDir()); !errors.Is(err, forge.ErrNoForge) {
		t.Errorf("404: error = %v, want ErrNoForge", err)
	}
}

// Gitea pages at fifty; the open list is read to #358's hundred and no
// further.
func TestGiteaREST_PagesToAHundred_issue459(t *testing.T) {
	api := &FakeGiteaAPI{Full: true}

	prs, err := restGitea(t, nil, api).ListPRs(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}
	open, pages := 0, 0
	for _, pr := range prs {
		if pr.State == forge.Open {
			open++
		}
	}
	for _, got := range api.Got {
		if strings.Contains(got.Path, "pulls?state=open") {
			pages++
		}
	}
	if open != 100 || pages != 2 {
		t.Errorf("read %d open pull requests over %d pages, want 100 over 2", open, pages)
	}
}

// A tea with no login for the host is passed over for REST, not run.
func TestGiteaREST_ATeaWithoutALoginIsPassedOver_issue459(t *testing.T) {
	bin, calls := fakeTea(t, `[]`)
	api := &FakeGiteaAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://codeberg.org/forgejo/forgejo.git"}).url},
		Bins:    map[forge.Kind]string{forge.KindGitea: bin}, API: api.serve(t),
	})

	if _, err := r.ListIssues(t.TempDir()); err != nil || len(api.Got) == 0 {
		t.Fatalf("ListIssues = %v after %d API calls, want it read over REST", err, len(api.Got))
	}
	if b, _ := os.ReadFile(calls); strings.Contains(string(b), "api") {
		t.Errorf("tea api was run with no login for the host:\n%s", b)
	}
}

// giteaAt is a Router over url on a self-hosted Gitea, with bins and env.
func giteaAt(t *testing.T, url string, bins map[forge.Kind]string, env map[string]string, api *FakeGiteaAPI) *forge.Router {
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: url}).url, Hosts: forge.Hosts{"git.corp.example": forge.KindGitea}},
		Bins:    bins, Env: env, API: api.serve(t),
	})
}

// A token never crosses plain http (#584): an http remote is read
// anonymously even with one set for it, and a refusal's note names tea alone,
// since no token could help. This first stopped the project outright, which
// cost a public repository the anonymous read it could have had (#589's
// review); what #584 fixed - no token sent, no "?" forever - still holds.
func TestGiteaREST_AnHTTPRemoteNeverCarriesTheToken_issue584(t *testing.T) {
	env := map[string]string{"GITEA_TOKEN": secret, "GITEA_INSTANCE_URL": "http://git.corp.example"}
	api := &FakeGiteaAPI{}
	if _, err := giteaAt(t, "http://git.corp.example/o/r.git", nil, env, api).ListIssues(t.TempDir()); err != nil || len(api.Got) == 0 || api.Got[0].Auth != "" {
		t.Errorf("ListIssues = %v, sent %+v; want an anonymous read", err, api.Got)
	}
	_, err := giteaAt(t, "http://git.corp.example/o/r.git", nil, env, &FakeGiteaAPI{Status: 401}).ListIssues(t.TempDir())
	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "tea" || missing.TokenEnv != "" {
		t.Errorf("401: error = %v, want tea missing and no token named", err)
	}
}

// With no token, an http remote is read anonymously where it is: over http,
// not upgraded to an https an http-only host does not answer (#584).
func TestGiteaREST_AnHTTPRemoteIsReadOverHTTP_issue584(t *testing.T) {
	api := &FakeGiteaAPI{}
	_, _ = giteaAt(t, "http://git.corp.example/o/r.git", nil, nil, api).ListIssues(t.TempDir())

	if len(api.Got) == 0 || api.Got[0].Scheme != "http" || api.Got[0].Auth != "" {
		t.Errorf("sent %+v, want an anonymous read over http", api.Got)
	}
}

// Read anonymously by a machine whose tea has no login for the host, a
// refusal names the login, not a missing tea (#586) - and so does the forced
// CLI transport.
func TestGiteaREST_ARefusalNamesTeasMissingLogin_issue586(t *testing.T) {
	bin, _ := fakeTea(t, `[]`)
	bins := map[forge.Kind]string{forge.KindGitea: bin}
	_, anon := giteaAt(t, "https://git.corp.example/o/r.git", bins, nil, &FakeGiteaAPI{Status: 404}).ListPRs(t.TempDir())
	forced := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://codeberg.org/forgejo/forgejo.git"}).url, Transport: forge.TransportCLI},
		Bins:    bins,
	})
	_, cli := forced.ListPRs(t.TempDir())

	for name, err := range map[string]error{"anonymous 404": anon, "CLI forced": cli} {
		var missing *forge.MissingToolError
		if !errors.As(err, &missing) || missing.NoLoginFor == "" || !strings.HasPrefix(missing.TokenEnv, "GITEA_TOKEN") {
			t.Errorf("%s: error = %v, want tea's missing login and GITEA_TOKEN named", name, err)
		}
	}
}

// stops is whether err is one the UI stops a project on for the run.
func stops(err error) bool {
	var missing *forge.MissingToolError
	var refused *forge.AuthError
	return errors.As(err, &missing) || errors.As(err, &refused) || errors.Is(err, forge.ErrNoForge)
}

// tea's env login binds GITEA_TOKEN to the instance GITEA_INSTANCE_URL names,
// and omatty borrows it the same way: a token set for one instance is never
// sent to another, and without the instance it is not sent at all (#589's
// review: a corporate token went to Codeberg).
func TestGiteaREST_ATokenGoesOnlyToItsInstance_issue459(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"another instance": {"GITEA_TOKEN": secret, "GITEA_INSTANCE_URL": "https://git.corp.example"},
		"no instance":      {"GITEA_TOKEN": secret},
	} {
		api := &FakeGiteaAPI{}
		_, _ = restGitea(t, env, api).ListIssues(t.TempDir())
		if len(api.Got) == 0 || api.Got[0].Auth != "" {
			t.Errorf("%s: sent %+v, want an anonymous read of codeberg.org", name, api.Got)
		}
	}
}

// The most common clone, over ssh, reads its instance over https with the
// token, as an https clone does.
func TestGiteaREST_AnSSHRemoteReadsOverHTTPSWithItsToken_issue459(t *testing.T) {
	api := &FakeGiteaAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@codeberg.org:forgejo/forgejo.git"}).url},
		Env:     codebergToken, API: api.serve(t),
	})

	if _, err := r.ListIssues(t.TempDir()); err != nil || api.Got[0].Scheme != "https" || api.Got[0].Auth != "token "+secret {
		t.Errorf("ListIssues = %v, sent %+v; want the token over https", err, api.Got)
	}
}

// Read anonymously, a repository whose issues are off answers 404 on the
// list and 200 on itself: none to show, not a note asking for a token -
// Codeberg's mirrors are the common case.
func TestGiteaREST_AnonymousIssuesTurnedOffAreNone_issue459(t *testing.T) {
	api := &FakeGiteaAPI{First: []giteaAnswer{
		{match: "issues?state=open", status: 404, body: `{}`},
		{match: "/repos/forgejo/forgejo?", status: 200, body: `{"id": 73144}`},
	}}

	if issues, err := restGitea(t, nil, api).ListIssues(t.TempDir()); err != nil || len(issues) != 0 {
		t.Errorf("ListIssues = %v, %v; want none and no error", issues, err)
	}
}

// When the list's 404 cannot be judged - the repository itself answers 502 -
// it is an outage asked again next poll, never a stop for the run (#589's
// review), with a token or without.
func TestGiteaREST_AnOutageInTheUnitCheckIsAskedAgain_issue459(t *testing.T) {
	for name, env := range map[string]map[string]string{"anonymous": nil, "token": codebergToken} {
		api := &FakeGiteaAPI{First: []giteaAnswer{
			{match: "issues?state=open", status: 404, body: `{}`},
			{match: "/repos/forgejo/forgejo?", status: 502, body: `{}`},
		}}
		if _, err := restGitea(t, env, api).ListIssues(t.TempDir()); err == nil || stops(err) {
			t.Errorf("%s: error = %v, want an outage", name, err)
		}
	}
}

// A closed list that fails is not a card with no finished pull requests.
func TestGiteaREST_AClosedListFailureIsNotHidden_issue459(t *testing.T) {
	api := &FakeGiteaAPI{First: []giteaAnswer{{match: "pulls?state=closed", status: 500, body: `{}`}}}

	if _, err := restGitea(t, nil, api).ListPRs(t.TempDir()); err == nil {
		t.Error("ListPRs = nil error, want the closed list's failure")
	}
}

// A captive portal or a challenge page answers an anonymous read with HTML:
// no token was refused, so it is an outage, not a stop asking for one.
func TestGiteaREST_AnAnonymousPageIsAnOutage_issue459(t *testing.T) {
	api := &FakeGiteaAPI{First: []giteaAnswer{{match: "issues?state=open", status: 200, contentType: "text/html", body: "<html>wait</html>"}}}

	if _, err := restGitea(t, nil, api).ListIssues(t.TempDir()); err == nil || stops(err) {
		t.Errorf("error = %v, want an outage", err)
	}
}
