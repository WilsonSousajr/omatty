package forge_test

import (
	"errors"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
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
