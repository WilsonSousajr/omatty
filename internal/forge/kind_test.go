package forge_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
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
		if !errors.Is(err, forge.ErrNoForge) {
			t.Errorf("KindOf(%q) error = %v, want ErrNoForge", host, err)
			continue
		}
		if !strings.Contains(err.Error(), `"`+host+`"`) {
			t.Errorf("KindOf(%q) error = %q, want the host in it", host, err)
		}
	}
}
