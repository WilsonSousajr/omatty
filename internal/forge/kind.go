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

// KindOf names the forge a host belongs to from the built-in table. An Azure
// organisation's own host, org.visualstudio.com, is matched by its suffix. Any
// other host is ErrNoForge: quiet, and never a guess.
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
	return "", fmt.Errorf("forge: host %q is not a forge omatty knows: %w", host, ErrNoForge)
}
