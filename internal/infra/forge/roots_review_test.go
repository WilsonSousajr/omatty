package forge_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/infra/forge"
)

// twoGiteas is one host serving a Gitea at its root and a second at /two.
var twoGiteas = forge.Hosts{"h.example": forge.KindGitea, "h.example/two": forge.KindGitea}

// #585's review, I1: with two forges on one host, a token bound to the one
// under /two must not reach the one at the root, whose remote has no root of
// its own. Host and port alone matched it before.
func TestGiteaREST_ATokenForASiblingRootStaysThere_issue585(t *testing.T) {
	api := &FakeGiteaAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://h.example/forgejo/forgejo.git"}).url, Hosts: twoGiteas},
		Env:     map[string]string{"GITEA_TOKEN": secret, "GITEA_INSTANCE_URL": "https://h.example/two"}, API: api.serve(t),
	})
	_, _ = r.ListPRs(t.TempDir())
	for _, got := range api.Got {
		if strings.Contains(got.Auth, secret) {
			t.Fatalf("sent /two's token to the root instance: %+v", got)
		}
	}
}

// I1 for Data Center: BITBUCKET_DC_URL naming the sibling root does not bind
// the root's own project.
func TestBitbucketDC_ATokenForASiblingRootStaysThere_issue585(t *testing.T) {
	api := &FakeBitbucketDCAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://git.corp.example/scm/ops/platform.git"}).url,
			Hosts: forge.Hosts{"git.corp.example": forge.KindBitbucket, "git.corp.example/two": forge.KindBitbucket}},
		Env: map[string]string{"BITBUCKET_DC_TOKEN": secret, "BITBUCKET_DC_URL": "https://git.corp.example/two"}, API: api.serve(t),
	})
	_, err := r.ListPRs(t.TempDir())
	var missing *dforge.MissingToolError
	if !errors.As(err, &missing) || len(api.Got) != 0 {
		t.Errorf("error = %v, sent %d; want the token unbound and nothing sent", err, len(api.Got))
	}
}

// I2: tea's login is kept per instance, and an instance is its root too: the
// project under /two must not read through the root's login.
func TestGitea_TeaLoginIsKeptPerRoot_issue585(t *testing.T) {
	logins := `[{"name":"root","url":"https://h.example","ssh_host":"h.example"},{"name":"two","url":"https://h.example/two","ssh_host":"h.example"}]`
	bin, calls := fakeTea(t, logins)
	a, b := t.TempDir(), t.TempDir() // tea runs in the project's directory
	remotes := map[string]string{a: "https://h.example/forgejo/forgejo.git", b: "https://h.example/two/forgejo/forgejo.git"}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: func(root string) (string, error) { return remotes[root], nil }, Hosts: twoGiteas},
		Bins:    map[forge.Kind]string{forge.KindGitea: bin},
	})
	_, _ = r.ListPRs(a)
	_, _ = r.ListPRs(b)
	got, _ := os.ReadFile(filepath.Clean(calls))
	if !strings.Contains(string(got), "--login two") {
		t.Errorf("tea calls:\n%s\nwant the /two project read through the login named two", got)
	}
}

// I3: two forges on one host with the same slug are two projects to every
// cache: the second reads its own finished list.
func TestFinished_KeyedByRootToo_issue585(t *testing.T) {
	api := &FakeGiteaAPI{}
	remotes := map[string]string{"/a": "https://h.example/forgejo/forgejo.git", "/b": "https://h.example/two/forgejo/forgejo.git"}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: func(root string) (string, error) { return remotes[root], nil }, Hosts: twoGiteas},
		API:     api.serve(t),
	})
	_, _ = r.ListPRs("/a")
	_, _ = r.ListPRs("/b")
	for _, got := range api.Got {
		if strings.HasPrefix(got.Path, "/two/api/v1/repos/forgejo/forgejo/pulls?state=closed") {
			return
		}
	}
	t.Errorf("sent %+v, want /two's own finished list read", api.Got)
}

// M1: tea's ssh_host match is by host alone, so for a rooted remote it would
// take a login for another root on the same host.
func TestTeaLogin_SSHHostDoesNotSkipTheRoot_issue585(t *testing.T) {
	logins := []byte(`[{"name":"other","url":"https://h.example/other","ssh_host":"h.example"}]`)
	remote := forge.Remote{Scheme: "ssh", Host: "h.example", Root: []string{"two"}, Path: []string{"o", "r"}}
	if got := forge.LoginFor(logins, remote); got != "" {
		t.Errorf("LoginFor = %q, want none: the login is for /other, not /two", got)
	}
}

// M3: an ambiguous host is refused before any alias lookup, so a gh login for
// that host cannot quietly route the project to GitHub instead.
func TestRouter_AmbiguousHostIsNotRoutedThroughAnAlias_issue585(t *testing.T) {
	gh := filepath.Join(t.TempDir(), "gh")
	if err := os.WriteFile(gh, []byte("#!/bin/sh\necho token\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@h.example:o/r.git"}).url,
			Hosts: forge.Hosts{"h.example/gitlab": forge.KindGitLab, "h.example/gitea": forge.KindGitea}},
		GH: gh,
	})
	_, err := r.ListPRs(t.TempDir())
	var ambiguous *dforge.AmbiguousHostError
	if !errors.As(err, &ambiguous) {
		t.Errorf("ListPRs = %v, want the AmbiguousHostError", err)
	}
}
