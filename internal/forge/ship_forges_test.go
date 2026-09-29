package forge_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// FakeWriteAPI is a forge's REST API for #331's three actions: it answers
// "METHOD path-part" routes with canned JSON and keeps every request, with its
// method and its body, so a test can read what omatty asked the forge to do.
type FakeWriteAPI struct {
	Routes map[string]string // "POST /merge_requests" -> answer
	mu     sync.Mutex
	Got    []FakeWrite
}

// FakeWrite is one request as the fake received it.
type FakeWrite struct {
	Method, Path, Host string
	Body               map[string]any
}

func (f *FakeWriteAPI) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		pq := r.URL.EscapedPath() + "?" + r.URL.RawQuery
		f.mu.Lock()
		f.Got = append(f.Got, FakeWrite{Method: r.Method, Path: pq, Host: r.Header.Get("X-Original-Host"), Body: body})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		for route, answer := range f.Routes {
			method, part, _ := strings.Cut(route, " ")
			if r.Method != method || !strings.Contains(pq, part) {
				continue
			}
			if answer == "" { // an answer with nothing to say: Gitea's merge
				w.Header().Del("Content-Type")
			}
			_, _ = w.Write([]byte(answer))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// write is the request whose method and path hold method and part.
func (f *FakeWriteAPI) write(method, part string) (FakeWrite, bool) {
	for _, w := range f.Got {
		if w.Method == method && strings.Contains(w.Path, part) {
			return w, true
		}
	}
	return FakeWrite{}, false
}

// greenHead is the head commit the card showed passing: what a merge pins.
const greenHead = "c0ffee12c0ffee12c0ffee12c0ffee12c0ffee12"

// shipForge is one forge's end of #331, as a fake would see it.
type shipForge struct {
	name, remote string
	hosts        forge.Hosts
	env          map[string]string
	routes       map[string]string
	open         [2]string // method and path of the create call
	merge        [2]string
	pin          string // how the merge body names the head it merges, when the forge takes one
	number       int
	protect      string // the path-part a protection read asks
	protectedYes string // an answer that says the branch is protected
	protectedNo  string
	noDelete     string // the body field that keeps the source branch
	method       string // the repository's own merge method, as the body names it
}

var shipForges = []shipForge{
	{
		name: "GitLab", remote: "git@gitlab.com:group/app.git", env: map[string]string{"GITLAB_TOKEN": secret},
		routes: map[string]string{"POST /merge_requests": `{"iid": 17}`, "PUT /merge_requests/17/merge": `{"state": "merged"}`},
		open:   [2]string{"POST", "/projects/group%2Fapp/merge_requests"}, merge: [2]string{"PUT", "/merge_requests/17/merge"}, number: 17,
		protect: "/repository/branches/main", protectedYes: `{"protected": true}`, protectedNo: `{"protected": false}`,
		noDelete: "should_remove_source_branch", pin: `"sha":"` + greenHead + `"`,
	},
	{
		name: "Gitea", remote: "https://codeberg.org/owner/app.git", env: codebergToken,
		routes: map[string]string{
			"POST /pulls/17/merge": "", "POST /repos/owner/app/pulls": `{"number": 17}`,
			"GET /repos/owner/app?": `{"default_merge_style": "squash"}`,
		},
		open: [2]string{"POST", "/api/v1/repos/owner/app/pulls"}, merge: [2]string{"POST", "/pulls/17/merge"}, number: 17,
		protect: "/branches/main", protectedYes: `{"protected": true}`, protectedNo: `{"protected": false}`,
		noDelete: "delete_branch_after_merge", method: `"Do":"squash"`, pin: `"head_commit_id":"` + greenHead + `"`,
	},
	{
		name: "Bitbucket", remote: "git@bitbucket.org:ws/app.git", env: map[string]string{"BITBUCKET_TOKEN": secret},
		routes: map[string]string{
			"POST /pullrequests/17/merge": `{"state": "MERGED"}`, "POST /repositories/ws/app/pullrequests": `{"id": 17}`,
			"GET /pullrequests/17": `{"id": 17, "source": {"commit": {"hash": "` + greenHead[:12] + `"}}}`,
		},
		open: [2]string{"POST", "/2.0/repositories/ws/app/pullrequests"}, merge: [2]string{"POST", "/pullrequests/17/merge"}, number: 17,
		protect:      "/branch-restrictions",
		protectedYes: `{"values": [{"kind": "push", "branch_match_kind": "glob", "pattern": "ma*"}]}`,
		protectedNo:  `{"values": [{"kind": "push", "branch_match_kind": "glob", "pattern": "release/*"}]}`,
		noDelete:     "close_source_branch",
	},
	{
		name: "Bitbucket Data Center", remote: "https://git.corp.example/scm/ops/app.git", hosts: forge.Hosts{"git.corp.example": forge.KindBitbucket},
		env: dcToken,
		routes: map[string]string{
			"POST /pull-requests/17/merge":  `{"state": "MERGED"}`,
			"GET /pull-requests/17":         `{"id": 17, "version": 3, "fromRef": {"latestCommit": "` + greenHead + `"}}`,
			"POST /repos/app/pull-requests": `{"id": 17}`,
		},
		open: [2]string{"POST", "/rest/api/1.0/projects/ops/repos/app/pull-requests"}, merge: [2]string{"POST", "/pull-requests/17/merge?version=3"}, number: 17,
		protect:      "/rest/branch-permissions/2.0/projects/ops/repos/app/restrictions",
		protectedYes: `{"isLastPage": true, "values": [{"type": "read-only", "matcher": {"id": "refs/heads/main", "type": {"id": "BRANCH"}}}]}`,
		protectedNo:  `{"isLastPage": true, "values": [{"type": "read-only", "matcher": {"id": "refs/heads/release", "type": {"id": "BRANCH"}}}]}`,
	},
	{
		name: "Azure DevOps", remote: "https://dev.azure.com/org/Proj/_git/app", env: map[string]string{"AZURE_DEVOPS_EXT_PAT": secret},
		routes: map[string]string{
			"POST /pullrequests":     `{"pullRequestId": 17}`,
			"GET /pullrequests/17":   `{"pullRequestId": 17, "lastMergeSourceCommit": {"commitId": "abc123"}}`,
			"PATCH /pullrequests/17": `{"status": "completed"}`, "GET /git/repositories/app?": `{"id": "repo-guid"}`,
		},
		open: [2]string{"POST", "/org/Proj/_apis/git/repositories/app/pullrequests"}, merge: [2]string{"PATCH", "/pullrequests/17"}, number: 17,
		protect:      "/policy/configurations",
		protectedYes: `{"value": [{"isEnabled": true, "isBlocking": true}]}`,
		protectedNo:  `{"value": [{"isEnabled": false, "isBlocking": true}]}`,
		noDelete:     "completionOptions", pin: `"lastMergeSourceCommit":{"commitId":"` + greenHead + `"}`,
	},
	{
		name: "GitHub", remote: "git@github.com:owner/app.git", env: map[string]string{"GH_TOKEN": secret},
		routes: map[string]string{
			"POST /repos/owner/app/pulls": `{"number": 17}`, "PUT /pulls/17/merge": `{"merged": true}`,
			"GET /repos/owner/app?": `{"allow_merge_commit": false, "allow_squash_merge": true, "allow_rebase_merge": true}`,
		},
		open: [2]string{"POST", "/repos/owner/app/pulls"}, merge: [2]string{"PUT", "/repos/owner/app/pulls/17/merge"}, number: 17,
		protect: "/repos/owner/app/branches/main", protectedYes: `{"protected": true}`, protectedNo: `{"protected": false}`,
		method: `"merge_method":"squash"`, pin: `"sha":"` + greenHead + `"`,
	},
}

func (s shipForge) router(t *testing.T, api *FakeWriteAPI) *forge.Router {
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: s.remote}).url, Hosts: s.hosts},
		Env:     s.env, API: api.serve(t),
	})
}

// Every forge opens a change for head against base with the session's title,
// and says its number (#464).
func TestShip_EveryForgeOpensAChange_issue464(t *testing.T) {
	for _, s := range shipForges {
		api := &FakeWriteAPI{Routes: s.routes}

		n, err := s.router(t, api).CreatePR(t.TempDir(), "feat/parser", "main", "Parse the thing")

		if err != nil || n != s.number {
			t.Errorf("%s: CreatePR = %d, %v; want %d", s.name, n, err, s.number)
			continue
		}
		w, ok := api.write(s.open[0], s.open[1])
		if !ok || !strings.Contains(asJSON(w.Body), "feat/parser") || !strings.Contains(asJSON(w.Body), "Parse the thing") {
			t.Errorf("%s: sent %+v, want %s %s naming the branch and title", s.name, api.Got, s.open[0], s.open[1])
		}
	}
}

// Every forge merges the change as it is asked to - never on a later green,
// never deleting the branch (#331's bounds).
func TestShip_EveryForgeMergesWithoutDeletingOrWaiting_issue464(t *testing.T) {
	for _, s := range shipForges {
		api := &FakeWriteAPI{Routes: s.routes}

		merged, err := s.router(t, api).MergePR(t.TempDir(), s.number, greenHead)
		if err != nil || !merged {
			t.Errorf("%s: MergePR = %v, %v; want merged", s.name, merged, err)
			continue
		}
		w, ok := api.write(s.merge[0], s.merge[1])
		if !ok {
			t.Errorf("%s: sent %+v, want %s %s", s.name, api.Got, s.merge[0], s.merge[1])
			continue
		}
		s.checkMergeBody(t, asJSON(w.Body))
	}
}

// neverInAMerge is every way a forge's merge body could wait for green or
// delete a branch: #331's bounds, in each forge's own words.
var neverInAMerge = []string{
	`"auto_merge":true`, `"merge_when_pipeline_succeeds":true`, `"merge_when_checks_succeed":true`,
	`"deleteSourceBranch":true`, `"delete_branch_after_merge":true`, `"close_source_branch":true`,
	`"should_remove_source_branch":true`, `"autoCompleteSetBy"`,
}

func (s shipForge) checkMergeBody(t *testing.T, body string) {
	t.Helper()
	for _, never := range neverInAMerge {
		if strings.Contains(body, never) {
			t.Errorf("%s: the merge asked %s", s.name, never)
		}
	}
	if s.noDelete != "" && !strings.Contains(body, s.noDelete) {
		t.Errorf("%s: the merge body %s does not say to keep the branch (%s)", s.name, body, s.noDelete)
	}
	if s.pin != "" && !strings.Contains(body, s.pin) {
		t.Errorf("%s: the merge body %s does not pin the head that was green (%s)", s.name, body, s.pin)
	}
	if s.method != "" && !strings.Contains(body, s.method) {
		t.Errorf("%s: the merge body %s does not use the repository's own method (%s)", s.name, body, s.method)
	}
}

// Every forge reads protection from its own API: protected, not protected,
// and a read that fails is protected - never assume a branch is open (#331).
func TestShip_EveryForgeReadsProtectionAndFailsClosed_issue464(t *testing.T) {
	for _, s := range shipForges {
		for answer, want := range map[string]bool{s.protectedYes: true, s.protectedNo: false} {
			routes := map[string]string{"GET " + s.protect: answer}
			for k, v := range s.routes {
				routes[k] = v
			}
			got, err := s.router(t, &FakeWriteAPI{Routes: routes}).BranchProtected(t.TempDir(), "main")
			if err != nil || got != want {
				t.Errorf("%s: answered %s: BranchProtected = %v, %v; want %v", s.name, answer, got, err, want)
			}
		}
		got, err := s.router(t, &FakeWriteAPI{Routes: s.routes}).BranchProtected(t.TempDir(), "main")
		if !got || err == nil {
			t.Errorf("%s: an unreadable protection = %v, %v; want protected beside an error", s.name, got, err)
		}
	}
}

func asJSON(v map[string]any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// fakeWriter is a forge CLI that keeps its arguments and the body it was
// handed on stdin, prints status on stderr (tea's --include; "" for glab)
// and answers answer - for tea, after answering its login list.
func fakeWriter(t *testing.T, name, status, answer string) (bin, calls, body string) {
	t.Helper()
	dir := t.TempDir()
	calls, body = filepath.Join(dir, "calls"), filepath.Join(dir, "body")
	script := "#!/bin/sh\n" +
		`printf '%s\n' "$*" >> '` + calls + "'\n" +
		`if [ "$1 $2" = "logins list" ]; then echo '` + codebergLogin + `'; exit 0; fi` + "\n" +
		`if [ "$1 $2" = "api --help" ]; then exit 0; fi` + "\n" +
		`cat > '` + body + "'\n" +
		`[ -n '` + status + `' ] && echo '` + status + `' >&2` + "\n" +
		`echo '` + answer + "'\n"
	bin = filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin, calls, body
}

// glab and tea carry a write the way they carry a read: glab's --method with
// the JSON body on stdin through --input, tea's -X with -d @- (#464).
func TestShip_TheCLIsCarryAWrite_issue464(t *testing.T) {
	for _, c := range []struct {
		kind                               forge.Kind
		name, remote, status, answer, argv string
	}{
		{forge.KindGitLab, "glab", "git@gitlab.com:group/app.git", "", `{"iid": 17}`,
			"api --hostname gitlab.com --method POST projects/group%2Fapp/merge_requests --input - --header Content-Type: application/json"},
		{forge.KindGitea, "tea", "https://codeberg.org/owner/app.git", "HTTP/1.1 201 Created", `{"number": 17}`,
			"api --login codeberg --include -X POST -d @- /repos/owner/app/pulls"},
	} {
		bin, calls, body := fakeWriter(t, c.name, c.status, c.answer)
		r := forge.NewTestRouter(forge.TestEnv{
			Options: forge.Options{Remote: (&FakeRemote{URL: c.remote}).url},
			Bins:    map[forge.Kind]string{c.kind: bin},
		})
		if n, err := r.CreatePR(t.TempDir(), "feat/parser", "main", "Parse the thing"); err != nil || n != 17 {
			t.Errorf("%s: CreatePR = %d, %v", c.name, n, err)
		}
		if got := string(mustRead(t, calls)); !strings.Contains(got, c.argv) {
			t.Errorf("%s was called:\n%s\nwant %s", c.name, got, c.argv)
		}
		if got := string(mustRead(t, body)); !strings.Contains(got, `"feat/parser"`) || !strings.Contains(got, `"Parse the thing"`) {
			t.Errorf("%s was handed %q, want the branch and the title as JSON", c.name, got)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
