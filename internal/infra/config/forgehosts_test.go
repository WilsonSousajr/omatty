package config_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/infra/config"
)

// [forge.hosts] names a self-hosted forge omatty cannot recognise by its
// hostname, forgejo spelled as the alias it is (#451).
func TestLoad_ForgeHostsNamesSelfHostedForges_issue451(t *testing.T) {
	home := t.TempDir()
	path := writeConfig(t, home, "[forge.hosts]\n\"git.corp.example\" = \"gitlab\"\n\"code.internal\" = \"forgejo\"\n\"ghe.corp.example\" = \"github\"\n")

	got, err := config.Load(path, home)

	if err != nil {
		t.Fatal(err)
	}
	for host, want := range map[string]forge.Kind{
		"git.corp.example": forge.KindGitLab, "code.internal": forge.KindGitea, "ghe.corp.example": forge.KindGitHub,
	} {
		if got.Forge.Hosts[host] != want {
			t.Errorf("forge.hosts[%q] = %q, want %q", host, got.Forge.Hosts[host], want)
		}
	}
}

// An unknown kind is a config error naming the file, the value and the kinds
// that would have worked.
func TestLoad_AnUnknownForgeKindNamesTheValueAndTheAcceptedSet_issue451(t *testing.T) {
	home := t.TempDir()
	path := writeConfig(t, home, "[forge.hosts]\n\"git.corp.example\" = \"gitlabx\"\n")

	_, err := config.Load(path, home)

	if err == nil {
		t.Fatal("Load() = nil error, want a refusal of gitlabx")
	}
	for _, want := range []string{path, `"gitlabx"`, "gitlab", "forgejo", "bitbucket"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %s", err, want)
		}
	}
}

// An empty section is no hosts, not an error.
func TestLoad_AnEmptyForgeHostsSectionIsFine_issue451(t *testing.T) {
	home := t.TempDir()

	got, err := config.Load(writeConfig(t, home, "[forge.hosts]\n"), home)

	if err != nil || len(got.Forge.Hosts) != 0 {
		t.Errorf("Load() = %+v, %v; want no hosts and no error", got.Forge, err)
	}
}

// A key is a host name as a remote's URL carries it: a scheme, a port or a
// path in it would never match, and a silent no-match is the #44 failure.
func TestLoad_AForgeHostThatIsNotAHostNameIsRefused_issue451(t *testing.T) {
	for _, key := range []string{"https://git.corp.example", "git.corp.example:8443", "git.corp.example/group", ""} {
		home := t.TempDir()
		path := writeConfig(t, home, "[forge.hosts]\n\""+key+"\" = \"gitlab\"\n")

		_, err := config.Load(path, home)

		if err == nil || !strings.Contains(err.Error(), "forge.hosts") || !strings.Contains(err.Error(), `"`+key+`"`) {
			t.Errorf("key %q: error = %v, want forge.hosts and the key named", key, err)
		}
	}
}

// Regression: [forge] hosts written as an array or a string - not a table -
// decoded to nothing and no error (a BurntSushi map-decode quirk), so the
// operator's line silently did nothing: #44's failure.
func TestLoad_ForgeHostsThatIsNotATableIsRefused_issue451(t *testing.T) {
	for _, body := range []string{
		"[forge]\nhosts = [\"git.corp.example\"]\n",
		"[forge]\nhosts = \"git.corp.example\"\n",
	} {
		home := t.TempDir()
		path := writeConfig(t, home, body)

		_, err := config.Load(path, home)

		if err == nil || !strings.Contains(err.Error(), "forge.hosts") || !strings.Contains(err.Error(), path) {
			t.Errorf("%q: error = %v, want forge.hosts refused, naming the file", body, err)
		}
	}
}

// Regression: two keys that differ only in case were both accepted, and which
// one named the host was map order - the forge could change between polls.
func TestLoad_ForgeHostsDifferingOnlyInCaseAreRefused_issue451(t *testing.T) {
	home := t.TempDir()
	path := writeConfig(t, home, "[forge.hosts]\n\"Git.Corp.Example\" = \"gitlab\"\n\"git.corp.example\" = \"gitea\"\n")

	_, err := config.Load(path, home)

	if err == nil || !strings.Contains(strings.ToLower(err.Error()), `"git.corp.example"`) {
		t.Errorf("error = %v, want the duplicated host named", err)
	}
}

// A key is refused unless it is a host name as DNS writes one: a trailing dot,
// an "@", a space or a tab would never match a remote's host.
func TestLoad_AForgeHostMustBeAHostName_issue451(t *testing.T) {
	for _, key := range []string{"git.corp.example.", "git@corp", "git corp", "git\\tcorp", ".corp"} {
		home := t.TempDir()
		path := writeConfig(t, home, "[forge.hosts]\n\""+key+"\" = \"gitlab\"\n")

		if _, err := config.Load(path, home); err == nil {
			t.Errorf("key %q loaded, want it refused", key)
		}
	}
}
