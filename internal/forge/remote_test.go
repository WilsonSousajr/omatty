package forge_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// Every shape a remote takes on every forge omatty reads: https, ssh:// and
// scp-style, with and without .git, ports, GitLab's nested groups, and Azure's
// three shapes. Path is the repository's segments, whatever the shape (#450).
func TestParseRemote_ReadsEveryShapeOfEveryForge_issue450(t *testing.T) {
	for _, tt := range []struct {
		raw  string
		want forge.Remote
	}{
		{"https://github.com/WilsonSousajr/omatty.git", forge.Remote{Scheme: "https", Host: "github.com", Path: []string{"WilsonSousajr", "omatty"}}},
		{"https://github.com/WilsonSousajr/omatty", forge.Remote{Scheme: "https", Host: "github.com", Path: []string{"WilsonSousajr", "omatty"}}},
		{"https://github.com/WilsonSousajr/omatty/", forge.Remote{Scheme: "https", Host: "github.com", Path: []string{"WilsonSousajr", "omatty"}}},
		{"git@github.com:WilsonSousajr/omatty.git", forge.Remote{Scheme: "ssh", Host: "github.com", Path: []string{"WilsonSousajr", "omatty"}}},
		{"ssh://git@github.com/WilsonSousajr/omatty.git", forge.Remote{Scheme: "ssh", Host: "github.com", Path: []string{"WilsonSousajr", "omatty"}}},
		{"ssh://git@ssh.github.com:443/WilsonSousajr/omatty.git", forge.Remote{Scheme: "ssh", Host: "ssh.github.com", Port: "443", Path: []string{"WilsonSousajr", "omatty"}}},
		{"https://gitlab.com/gitlab-org/cli.git", forge.Remote{Scheme: "https", Host: "gitlab.com", Path: []string{"gitlab-org", "cli"}}},
		{"git@gitlab.com:group/sub/deeper/project.git", forge.Remote{Scheme: "ssh", Host: "gitlab.com", Path: []string{"group", "sub", "deeper", "project"}}},
		{"ssh://git@git.corp.example:2222/group/sub/project.git", forge.Remote{Scheme: "ssh", Host: "git.corp.example", Port: "2222", Path: []string{"group", "sub", "project"}}},
		{"https://git.corp.example:8443/group/project", forge.Remote{Scheme: "https", Host: "git.corp.example", Port: "8443", Path: []string{"group", "project"}}},
		{"http://gitea.local:3000/owner/repo.git", forge.Remote{Scheme: "http", Host: "gitea.local", Port: "3000", Path: []string{"owner", "repo"}}},
		{"https://codeberg.org/forgejo/forgejo.git", forge.Remote{Scheme: "https", Host: "codeberg.org", Path: []string{"forgejo", "forgejo"}}},
		{"git@bitbucket.org:atlassianlabs/atlascode.git", forge.Remote{Scheme: "ssh", Host: "bitbucket.org", Path: []string{"atlassianlabs", "atlascode"}}},
		{"https://GitHub.com/WilsonSousajr/omatty", forge.Remote{Scheme: "https", Host: "github.com", Path: []string{"WilsonSousajr", "omatty"}}},
		// Azure DevOps: https with and without the org as a user, ssh's v3
		// prefix dropped, and the old visualstudio.com host with and without
		// its DefaultCollection.
		{"https://dev.azure.com/org/Project/_git/repo", forge.Remote{Scheme: "https", Host: "dev.azure.com", Path: []string{"org", "Project", "_git", "repo"}}},
		{"https://org@dev.azure.com/org/Project/_git/repo", forge.Remote{Scheme: "https", Host: "dev.azure.com", Path: []string{"org", "Project", "_git", "repo"}}},
		{"git@ssh.dev.azure.com:v3/org/Project/repo", forge.Remote{Scheme: "ssh", Host: "ssh.dev.azure.com", Path: []string{"org", "Project", "repo"}}},
		{"https://org.visualstudio.com/Project/_git/repo", forge.Remote{Scheme: "https", Host: "org.visualstudio.com", Path: []string{"Project", "_git", "repo"}}},
		{"https://org.visualstudio.com/DefaultCollection/Project/_git/repo", forge.Remote{Scheme: "https", Host: "org.visualstudio.com", Path: []string{"DefaultCollection", "Project", "_git", "repo"}}},
		{"org@vs-ssh.visualstudio.com:v3/org/Project/repo", forge.Remote{Scheme: "ssh", Host: "vs-ssh.visualstudio.com", Path: []string{"org", "Project", "repo"}}},
	} {
		got, err := forge.ParseRemote(tt.raw)
		if err != nil {
			t.Errorf("ParseRemote(%q) error = %v", tt.raw, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseRemote(%q) = %+v, want %+v", tt.raw, got, tt.want)
		}
	}
}

// A malformed remote is an error that carries the value and the shapes that
// would have been read, so the operator can see what to fix.
func TestParseRemote_RefusesWhatIsNotARemoteAndSaysWhy_issue450(t *testing.T) {
	for _, raw := range []string{
		"",
		"not a url",
		"https://github.com",
		"https://github.com/",
		"git@github.com:",
		"file:///srv/git/repo.git",
		"/srv/git/repo.git",
		"../sibling/repo",
		"ftp://example.com/owner/repo",
	} {
		_, err := forge.ParseRemote(raw)
		if err == nil {
			t.Errorf("ParseRemote(%q) = nil error, want a refusal", raw)
			continue
		}
		if msg := err.Error(); !strings.Contains(msg, fmt.Sprintf("%q", raw)) || !strings.Contains(msg, "want") {
			t.Errorf("ParseRemote(%q) error = %q, want the value and the expected shape", raw, msg)
		}
	}
}

// A token in a remote's URL is a credential: the parsed Remote holds none of
// it, and an error about the URL never repeats it.
func TestParseRemote_NeverKeepsOrRepeatsACredential_issue450(t *testing.T) {
	got, err := forge.ParseRemote(withUserinfo("https://", "oauth2:SECRET", "gitlab.com/group/project.git"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Host+strings.Join(got.Path, "/"), "SECRET") {
		t.Errorf("the parsed remote kept the token: %+v", got)
	}
	_, err = forge.ParseRemote(withUserinfo("https://", "oauth2:SECRET", "gitlab.com"))
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("error = %v, want a refusal that does not repeat the token", err)
	}
}

// withUserinfo builds a remote with a credential in it at run time: a literal
// user:secret@host in the source reads to a secret scanner as a leaked
// credential, and these are fakes.
func withUserinfo(scheme, userinfo, rest string) string { return scheme + userinfo + "@" + rest }

// Slug is the repository's path as its forge's API addresses it.
func TestRemote_SlugJoinsThePath_issue450(t *testing.T) {
	r := forge.Remote{Host: "gitlab.com", Path: []string{"group", "sub", "project"}}
	if got := r.Slug(); got != "group/sub/project" {
		t.Errorf("Slug() = %q, want group/sub/project", got)
	}
}
