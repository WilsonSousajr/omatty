package forge_test

import (
	"errors"
	"strings"
	"testing"

	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/infra/forge"
)

var rootedGitLab = forge.Hosts{"h.example/gitlab": forge.KindGitLab}

// #585: a GitLab under https://h/gitlab is asked at /gitlab/api/v4, for the
// project the path names after the root - over https and over ssh, which
// carries no root of its own.
func TestGitLabREST_UnderARelativeRoot_issue585(t *testing.T) {
	for _, url := range []string{"https://h.example/gitlab/gitlab-org/cli.git", "git@h.example:gitlab-org/cli.git"} {
		api := &FakeGitLabAPI{}
		if _, err := restGitLab(t, url, api, rootedGitLab).ListPRs(t.TempDir()); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
		if got := api.Got[0]; got.Path != "/gitlab/api/v4/projects/gitlab-org%2Fcli/merge_requests" || got.Host != "h.example" {
			t.Errorf("%s: first request %+v, want h.example /gitlab/api/v4/projects/gitlab-org%%2Fcli/merge_requests", url, got)
		}
	}
}

// glab's --hostname takes no path, so it would ask the host's root for the
// project: a rooted GitLab is read over REST alone, glab installed or not.
func TestGitLab_RootedSkipsGlab_issue585(t *testing.T) {
	bin, calls := fakeGlab(t)
	api := &FakeGitLabAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://h.example/gitlab/gitlab-org/cli.git"}).url, Hosts: rootedGitLab},
		Bins:    map[forge.Kind]string{forge.KindGitLab: bin},
		Env:     map[string]string{"GITLAB_TOKEN": secret}, API: api.serve(t),
	})
	if _, err := r.ListPRs(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if got := glabCalls(t, calls); len(got) != 0 {
		t.Errorf("glab ran %v, want REST alone for a rooted GitLab", got)
	}
}

// With no token there is no way in: glab would read the wrong place, so the
// note names the token alone.
func TestGitLab_RootedWithoutTokenIsMissing_issue585(t *testing.T) {
	bin, _ := fakeGlab(t)
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://h.example/gitlab/gitlab-org/cli.git"}).url, Hosts: rootedGitLab},
		Bins:    map[forge.Kind]string{forge.KindGitLab: bin},
	})
	_, err := r.ListPRs(t.TempDir())
	var missing *dforge.MissingToolError
	if !errors.As(err, &missing) || missing.TokenEnv != "GITLAB_TOKEN" || missing.Tool != "" {
		t.Errorf("error = %v, want GITLAB_TOKEN named alone", err)
	}
}

// Gitea under a root: asked there, with the token GITEA_INSTANCE_URL binds
// to that root and to no other.
func TestGiteaREST_UnderARelativeRoot_issue585(t *testing.T) {
	for instance, wantToken := range map[string]bool{"https://h.example/gitea": true, "https://h.example/other": false} {
		api := &FakeGiteaAPI{}
		r := forge.NewTestRouter(forge.TestEnv{
			Options: forge.Options{Remote: (&FakeRemote{URL: "https://h.example/gitea/forgejo/forgejo.git"}).url, Hosts: forge.Hosts{"h.example/gitea": forge.KindGitea}},
			Env:     map[string]string{"GITEA_TOKEN": secret, "GITEA_INSTANCE_URL": instance}, API: api.serve(t),
		})
		_, _ = r.ListPRs(t.TempDir())
		if len(api.Got) == 0 || !strings.HasPrefix(api.Got[0].Path, "/gitea/api/v1/repos/forgejo/forgejo/pulls") {
			t.Fatalf("%s: sent %+v, want /gitea/api/v1/repos/forgejo/forgejo/pulls first", instance, api.Got)
		}
		if got := strings.Contains(api.Got[0].Auth, secret); got != wantToken {
			t.Errorf("GITEA_INSTANCE_URL=%s: token sent = %v, want %v", instance, got, wantToken)
		}
	}
}

// The path binds a token only for a rooted remote; one with no root keeps the
// host-and-port rule it always had, so a context path in BITBUCKET_DC_URL or a
// tea login under a subpath keeps working.
func TestNamesInstance_ComparesThePathOnlyWhenRooted_issue585(t *testing.T) {
	rooted := forge.Remote{Scheme: "https", Host: "h.example", Root: []string{"gitlab"}, Path: []string{"g", "p"}}
	bare := forge.Remote{Scheme: "ssh", Host: "h.example", Path: []string{"g", "p"}}
	for _, c := range []struct {
		url    string
		remote forge.Remote
		want   bool
	}{
		{"https://h.example/gitlab", rooted, true},
		{"https://h.example/gitlab/", rooted, true},
		{"https://h.example", rooted, false},
		{"https://h.example/other", rooted, false},
		{"https://h.example/bitbucket", bare, true},
	} {
		if got := forge.NamesInstance(c.url, c.remote); got != c.want {
			t.Errorf("NamesInstance(%q, root %v) = %v, want %v", c.url, c.remote.Root, got, c.want)
		}
	}
}

// Regression: an ssh Data Center clone takes its context path from
// BITBUCKET_DC_URL, with no rooted key at all (#461). #585 leaves it working.
func TestBitbucketDC_SSHContextPathStillBinds_issue585(t *testing.T) {
	api := &FakeBitbucketDCAPI{}
	env := map[string]string{"BITBUCKET_DC_TOKEN": secret, "BITBUCKET_DC_URL": "https://git.corp.example/bitbucket"}
	_, _ = dcRouter(t, "ssh://git@git.corp.example:7999/ops/platform.git", env, api).ListPRs(t.TempDir())
	if len(api.Got) == 0 || !strings.HasPrefix(api.Got[0].Path, "/bitbucket/rest/api/1.0/projects/ops/repos/platform/") {
		t.Errorf("sent %+v, want the variable's context path", api.Got)
	}
}

// Under a rooted key a Data Center's https clone keeps its context path once,
// not twice: the root is the context path.
func TestBitbucketDC_UnderARootedKey_issue585(t *testing.T) {
	api := &FakeBitbucketDCAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://git.corp.example/bitbucket/scm/ops/platform.git"}).url,
			Hosts: forge.Hosts{"git.corp.example/bitbucket": forge.KindBitbucket}},
		Env: map[string]string{"BITBUCKET_DC_TOKEN": secret, "BITBUCKET_DC_URL": "https://git.corp.example/bitbucket"}, API: api.serve(t),
	})
	_, _ = r.ListPRs(t.TempDir())
	if len(api.Got) == 0 || !strings.HasPrefix(api.Got[0].Path, "/bitbucket/rest/api/1.0/projects/ops/repos/platform/") {
		t.Errorf("sent %+v, want /bitbucket/rest once", api.Got)
	}
}

// A rooted GitLab over plain http is refused before any token goes out
// (#584's rule): there is no glab to hand it to here.
func TestGitLab_RootedOverHTTPSendsNoToken_issue585(t *testing.T) {
	api := &FakeGitLabAPI{}
	_, err := restGitLab(t, "http://h.example/gitlab/gitlab-org/cli.git", api, rootedGitLab).ListPRs(t.TempDir())
	var plain *dforge.PlainHTTPError
	if !errors.As(err, &plain) || len(api.Got) != 0 {
		t.Errorf("error = %v, sent %d requests; want PlainHTTPError and nothing sent", err, len(api.Got))
	}
}
