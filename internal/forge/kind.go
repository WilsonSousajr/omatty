package forge

import (
	"fmt"
	"strings"
)

// Kind is which forge a repository lives on. It is derived from the remote's
// host at runtime and never persisted (invariant 9).
//
//	kind, err := forge.KindOf("gitlab.com") // forge.KindGitLab
type Kind string

// The forges omatty reads (M16). Forgejo and Codeberg are KindGitea: they share
// its API.
const (
	KindGitHub    Kind = "github"
	KindGitLab    Kind = "gitlab"
	KindAzure     Kind = "azure"
	KindGitea     Kind = "gitea"
	KindBitbucket Kind = "bitbucket"
)

// builtInHosts is every public forge's host. A self-hosted one cannot be told
// from its name - git.corp.example is GitLab as often as Gitea - so it is named
// by the operator, never guessed at (#451).
var builtInHosts = map[string]Kind{
	"github.com":        KindGitHub,
	"ssh.github.com":    KindGitHub, // GitHub's ssh over port 443
	"gitlab.com":        KindGitLab,
	"altssh.gitlab.com": KindGitLab, // GitLab's ssh over port 443
	"dev.azure.com":     KindAzure,
	"ssh.dev.azure.com": KindAzure,
	"codeberg.org":      KindGitea,
	"gitea.com":         KindGitea,
	"bitbucket.org":     KindBitbucket,
}

// sshOver443 is each forge's ssh-over-https-port host and the host every API
// and web page is on: git reaches it, nothing else does (#454's review).
var sshOver443 = map[string]string{"ssh.github.com": "github.com", "altssh.gitlab.com": "gitlab.com"}

// webHost is remote as everything but git sees it: an ssh-over-443 host
// becomes its forge's own, without ssh's port.
func webHost(remote Remote) Remote {
	if host, ok := sshOver443[remote.Host]; ok {
		remote.Host, remote.Port = host, ""
	}
	return remote
}

// KindOf names the forge a host belongs to from the built-in table. An Azure
// organisation's own host, org.visualstudio.com, is matched by its suffix. Any
// other host is ErrNoForge: quiet, and never a guess - the error says where to
// name it.
//
//	kind, err := forge.KindOf(remote.Host)
func KindOf(host string) (Kind, error) {
	host = strings.ToLower(host)
	if kind, ok := builtInHosts[host]; ok {
		return kind, nil
	}
	if strings.HasSuffix(host, ".visualstudio.com") {
		return KindAzure, nil
	}
	return "", fmt.Errorf("forge: host %q is not a forge omatty knows; name it in [forge.hosts]: %w", host, ErrNoForge)
}

// kindNames is every spelling a config may use, in the order an error lists
// them. forgejo is Gitea's fork and speaks its API, so it is an alias.
var kindNames = map[string]Kind{
	"github": KindGitHub, "gitlab": KindGitLab, "azure": KindAzure,
	"gitea": KindGitea, "forgejo": KindGitea, "bitbucket": KindBitbucket,
}

// acceptedKinds is kindNames for a message, in a fixed order.
const acceptedKinds = "github, gitlab, azure, gitea, forgejo or bitbucket"

// ParseKind reads a kind as a person writes it in [forge.hosts] (#451).
//
//	kind, err := forge.ParseKind("forgejo") // forge.KindGitea
func ParseKind(s string) (Kind, error) {
	if kind, ok := kindNames[s]; ok {
		return kind, nil
	}
	return "", fmt.Errorf("forge: kind %q is not a forge omatty reads, want %s", s, acceptedKinds)
}

// UnmarshalText decodes a kind from config text through ParseKind, so a bad
// kind fails where the file is read and the kind list is written once.
func (k *Kind) UnmarshalText(text []byte) error {
	kind, err := ParseKind(string(text))
	if err != nil {
		return err
	}
	*k = kind
	return nil
}

// Hosts is the operator's own host table, [forge.hosts]: a self-managed
// GitLab, a Forgejo, a GitHub Enterprise - hosts no name can identify (#451).
//
//	forge.Hosts{"git.corp.example": forge.KindGitLab}.KindOf("git.corp.example")
type Hosts map[string]Kind

// KindOf reads the operator's table before the built-in one, so a built-in
// host can be overridden too. Hosts are compared without regard to case, the
// way DNS compares them. A nil table is the built-in one alone.
func (h Hosts) KindOf(host string) (Kind, error) {
	for name, kind := range h {
		if strings.EqualFold(name, host) {
			return kind, nil
		}
	}
	return KindOf(host)
}
