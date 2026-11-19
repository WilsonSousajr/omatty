package forge_test

import (
	"errors"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"reflect"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/forge"
)

// The built-in table names every public forge's host, and an Azure
// organisation's own visualstudio.com host, by its suffix (#450).
func TestKindOf_NamesEveryBuiltInHost_issue450(t *testing.T) {
	for host, want := range map[string]forge.Kind{
		"github.com":              forge.KindGitHub,
		"ssh.github.com":          forge.KindGitHub,
		"gitlab.com":              forge.KindGitLab,
		"altssh.gitlab.com":       forge.KindGitLab,
		"dev.azure.com":           forge.KindAzure,
		"ssh.dev.azure.com":       forge.KindAzure,
		"myorg.visualstudio.com":  forge.KindAzure,
		"vs-ssh.visualstudio.com": forge.KindAzure,
		"codeberg.org":            forge.KindGitea,
		"gitea.com":               forge.KindGitea,
		"bitbucket.org":           forge.KindBitbucket,
		"GitHub.com":              forge.KindGitHub,
	} {
		got, err := forge.KindOf(host)
		if err != nil || got != want {
			t.Errorf("KindOf(%q) = %q, %v, want %q", host, got, err, want)
		}
	}
}

// An unknown host is ErrNoForge, never a guess: a self-hosted GitLab and a
// self-hosted Gitea look the same from their names.
func TestKindOf_AnUnknownHostIsErrNoForgeAndNamesIt_issue450(t *testing.T) {
	for _, host := range []string{"git.corp.example", "visualstudio.com.evil.example", "evilvisualstudio.com", "notgithub.com", ""} {
		_, err := forge.KindOf(host)
		if !errors.Is(err, dforge.ErrNoForge) {
			t.Errorf("KindOf(%q) error = %v, want ErrNoForge", host, err)
			continue
		}
		if !strings.Contains(err.Error(), `"`+host+`"`) {
			t.Errorf("KindOf(%q) error = %q, want the host in it", host, err)
		}
	}
}

// Every kind a config can name, and forgejo as the alias it is: Forgejo is a
// Gitea fork and speaks its API (#451).
func TestParseKind_AcceptsEveryKindAndForgejoAsGitea_issue451(t *testing.T) {
	for s, want := range map[string]forge.Kind{
		"github": forge.KindGitHub, "gitlab": forge.KindGitLab, "azure": forge.KindAzure,
		"gitea": forge.KindGitea, "forgejo": forge.KindGitea, "bitbucket": forge.KindBitbucket,
	} {
		if got, err := forge.ParseKind(s); err != nil || got != want {
			t.Errorf("ParseKind(%q) = %q, %v, want %q", s, got, err, want)
		}
	}
}

// An unknown kind names the value and the accepted set, so the fix is in the
// message.
func TestParseKind_RefusesAnUnknownKindNamingTheAcceptedSet_issue451(t *testing.T) {
	_, err := forge.ParseKind("gitlabx")
	if err == nil {
		t.Fatal("ParseKind(gitlabx) = nil error, want a refusal")
	}
	for _, want := range []string{`"gitlabx"`, "github", "gitlab", "azure", "gitea", "forgejo", "bitbucket"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %s", err, want)
		}
	}
}

// The operator's table is read before the built-in one, so a self-hosted host
// is named and a built-in one can be overridden; case does not matter.
func TestHosts_KindOfPrefersTheConfigOverTheBuiltInTable_issue451(t *testing.T) {
	hosts := forge.Hosts{"Git.Corp.Example": forge.KindGitLab, "gitea.com": forge.KindGitHub}
	for host, want := range map[string]forge.Kind{
		"git.corp.example": forge.KindGitLab,
		"gitea.com":        forge.KindGitHub,
		"codeberg.org":     forge.KindGitea,
	} {
		if got, err := hosts.KindOf(host); err != nil || got != want {
			t.Errorf("KindOf(%q) = %q, %v, want %q", host, got, err, want)
		}
	}
}

// With nothing configured the built-in table answers alone, and an unknown
// host says where to name it.
func TestHosts_NilIsTheBuiltInTableAndPointsAtTheConfig_issue451(t *testing.T) {
	var hosts forge.Hosts
	if got, err := hosts.KindOf("gitlab.com"); err != nil || got != forge.KindGitLab {
		t.Errorf("KindOf(gitlab.com) = %q, %v, want gitlab", got, err)
	}
	_, err := hosts.KindOf("git.corp.example")
	if !errors.Is(err, dforge.ErrNoForge) || !strings.Contains(err.Error(), "[forge.hosts]") {
		t.Errorf("KindOf(git.corp.example) error = %v, want ErrNoForge pointing at [forge.hosts]", err)
	}
}

// Kind decodes from config text through ParseKind, so the kind list exists once.
func TestKind_UnmarshalTextIsParseKind_issue451(t *testing.T) {
	var k forge.Kind
	if err := k.UnmarshalText([]byte("forgejo")); err != nil || k != forge.KindGitea {
		t.Errorf("UnmarshalText(forgejo) = %q, %v, want gitea", k, err)
	}
	if err := k.UnmarshalText([]byte("svn")); err == nil {
		t.Error("UnmarshalText(svn) = nil error, want a refusal")
	}
}

// #585: a [forge.hosts] key is a host, then the root a forge is served under.
func TestSplitHostKey_issue585(t *testing.T) {
	for key, want := range map[string]struct {
		host string
		root []string
	}{
		"host.corp":          {"host.corp", nil},
		"host.corp/gitlab":   {"host.corp", []string{"gitlab"}},
		"host.corp/gitlab/x": {"host.corp", []string{"gitlab", "x"}},
	} {
		host, root := forge.SplitHostKey(key)
		if host != want.host || !reflect.DeepEqual(root, want.root) {
			t.Errorf("SplitHostKey(%q) = %q, %q, want %q, %q", key, host, root, want.host, want.root)
		}
	}
}

// mustRemote is raw parsed, or the test stops.
func mustRemote(t *testing.T, raw string) forge.Remote {
	t.Helper()
	r, err := forge.ParseRemote(raw)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// #585: the longest root that prefixes the remote's path wins, and its
// segments leave the path for Root, so the slug is the repository's alone.
func TestHosts_LongestRootWins_issue585(t *testing.T) {
	hosts := forge.Hosts{"h.example/a": forge.KindGitLab, "h.example/a/b": forge.KindGitea}
	got, kind, err := hosts.Resolve(mustRemote(t, "https://h.example/a/b/g/p.git"))
	if err != nil || kind != forge.KindGitea || !reflect.DeepEqual(got.Root, []string{"a", "b"}) || got.Slug() != "g/p" {
		t.Errorf("Resolve(a/b/g/p) = %+v, %q, %v; want gitea, root a/b, slug g/p", got, kind, err)
	}
	got, kind, err = hosts.Resolve(mustRemote(t, "https://H.example/a/g/p"))
	if err != nil || kind != forge.KindGitLab || !reflect.DeepEqual(got.Root, []string{"a"}) || got.Slug() != "g/p" {
		t.Errorf("Resolve(a/g/p) = %+v, %q, %v; want gitlab, root a, slug g/p", got, kind, err)
	}
}

// A root that would leave no repository is not a match: "https://h/a" is a
// repository named a on the host, not the root of one.
func TestHosts_RootMustLeaveARepository_issue585(t *testing.T) {
	hosts := forge.Hosts{"h.example/a": forge.KindGitLab}
	if _, _, err := hosts.Resolve(mustRemote(t, "https://h.example/a")); err == nil {
		t.Error("Resolve(https://h.example/a) = nil error, want no match: the root leaves no repository")
	}
}

// A root is the server's path and keeps its case.
func TestHosts_RootIsCaseSensitive_issue585(t *testing.T) {
	hosts := forge.Hosts{"h.example/gitlab": forge.KindGitLab}
	if _, _, err := hosts.Resolve(mustRemote(t, "https://h.example/GitLab/g/p")); err == nil {
		t.Error("Resolve(/GitLab/...) matched /gitlab, want a path compared as the server compares it")
	}
}

// An ssh remote has no root in its path; a host with one rooted key uses it.
func TestHosts_SSHUsesTheOnlyRoot_issue585(t *testing.T) {
	got, kind, err := forge.Hosts{"h.example/gitlab": forge.KindGitLab}.Resolve(mustRemote(t, "git@h.example:g/p.git"))
	if err != nil || kind != forge.KindGitLab || !reflect.DeepEqual(got.Root, []string{"gitlab"}) || got.Slug() != "g/p" {
		t.Errorf("Resolve = %+v, %q, %v; want gitlab under root gitlab, slug g/p", got, kind, err)
	}
}

// A bare key says the ssh host is that forge, at its root: it wins.
func TestHosts_SSHPrefersABareKey_issue585(t *testing.T) {
	hosts := forge.Hosts{"h.example": forge.KindGitea, "h.example/gitlab": forge.KindGitLab}
	got, kind, err := hosts.Resolve(mustRemote(t, "git@h.example:g/p.git"))
	if err != nil || kind != forge.KindGitea || len(got.Root) != 0 {
		t.Errorf("Resolve = %+v, %q, %v; want the bare key's gitea with no root", got, kind, err)
	}
}

// Two rooted forges on one host cannot be told apart over ssh: refused aloud,
// never guessed.
func TestHosts_SSHWithTwoRootsRefusesAloud_issue585(t *testing.T) {
	hosts := forge.Hosts{"h.example/gitlab": forge.KindGitLab, "h.example/gitea": forge.KindGitea}
	_, _, err := hosts.Resolve(mustRemote(t, "git@h.example:g/p.git"))
	var ambiguous *dforge.AmbiguousHostError
	if !errors.As(err, &ambiguous) || !errors.Is(err, dforge.ErrNoForge) {
		t.Errorf("Resolve = %v, want an AmbiguousHostError that is ErrNoForge", err)
	}
}

// With no rooted key the answer is the host table's, as before.
func TestHosts_UnrootedIsUnchanged_issue585(t *testing.T) {
	got, kind, err := forge.Hosts{"git.corp.example": forge.KindGitLab}.Resolve(mustRemote(t, "https://git.corp.example/group/sub/p"))
	if err != nil || kind != forge.KindGitLab || len(got.Root) != 0 || got.Slug() != "group/sub/p" {
		t.Errorf("Resolve = %+v, %q, %v; want gitlab, no root, the whole path", got, kind, err)
	}
}

// The root goes back into every web and API URL, through webBase.
func TestRemote_WebBaseKeepsTheRoot_issue585(t *testing.T) {
	r := forge.Remote{Scheme: "ssh", Host: "h.example", Root: []string{"gitlab"}, Path: []string{"g", "p"}}
	if got := forge.WebBase(r); got != "https://h.example/gitlab" {
		t.Errorf("WebBase = %q, want https://h.example/gitlab", got)
	}
}
