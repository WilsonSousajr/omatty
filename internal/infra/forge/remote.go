package forge

import (
	"fmt"
	"net/url"
	"strings"
)

// Remote is a repository's address on its forge, read off a git remote's URL
// (#450). It holds no credential: a token in the URL is dropped on parsing.
//
//	r, err := forge.ParseRemote("git@gitlab.com:group/sub/project.git")
//	// r.Host "gitlab.com", r.Path ["group" "sub" "project"]
type Remote struct {
	// Scheme is "https", "http" or "ssh"; an scp-style remote is "ssh".
	Scheme string
	// Host is lowercased and carries no port.
	Host string
	// Port is the URL's explicit port, or "".
	Port string
	// Path is the repository's segments, without ".git". An Azure ssh remote
	// loses its "v3" prefix, so all three Azure shapes end org/project/repo
	// or project/_git/repo as their forge writes them.
	Path []string
}

// Slug is the repository's path as its forge's API addresses it.
//
//	forge.Remote{Path: []string{"group", "sub", "project"}}.Slug() // "group/sub/project"
func (r Remote) Slug() string { return strings.Join(r.Path, "/") }

// remoteShapes is what ParseRemote reads, for the error that refuses the rest.
const remoteShapes = "want https://host/owner/repo, ssh://git@host/owner/repo or git@host:owner/repo"

// ParseRemote reads a remote's URL in any of the three shapes git accepts for a
// hosted repository - https, ssh:// and scp-style - and refuses a local path or
// a URL with no repository in it. Pure: it never looks the host up.
//
//	r, err := forge.ParseRemote("https://dev.azure.com/org/Project/_git/repo")
func ParseRemote(raw string) (Remote, error) {
	r, ok := parseURL(raw)
	if !ok {
		r, ok = parseSCP(raw)
	}
	if !ok || r.Host == "" || len(r.Path) == 0 {
		return Remote{}, fmt.Errorf("forge: remote %q is not a repository omatty can read, %s", redact(raw), remoteShapes)
	}
	return r, nil
}

// parseURL reads a remote with a scheme. url.Parse accepts nearly anything, so
// the scheme is what decides: a file:// remote is a local path, not a forge.
func parseURL(raw string) (Remote, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return Remote{}, false
	}
	scheme, hosted := hostedSchemes[u.Scheme]
	if !hosted {
		return Remote{}, false
	}
	host := strings.ToLower(u.Hostname())
	return Remote{Scheme: scheme, Host: host, Port: u.Port(), Path: segments(host, u.Path)}, true
}

// hostedSchemes is every scheme git reaches a hosted repository by, and what
// it is: git+ssh and ssh+git are ssh by older names git still accepts.
var hostedSchemes = map[string]string{
	"https": "https", "http": "http", "ssh": "ssh", "git+ssh": "ssh", "ssh+git": "ssh",
}

// parseSCP reads git's scp-like shape, [user@]host:path. A path with a slash
// before its first colon is a local path, which is how git tells them apart.
func parseSCP(raw string) (Remote, bool) {
	if strings.Contains(raw, "://") {
		return Remote{}, false
	}
	hostPart, path, found := strings.Cut(raw, ":")
	if !found || strings.Contains(hostPart, "/") {
		return Remote{}, false
	}
	if _, after, hasUser := strings.Cut(hostPart, "@"); hasUser {
		hostPart = after
	}
	host := strings.ToLower(hostPart)
	return Remote{Scheme: "ssh", Host: host, Path: segments(host, path)}, true
}

// segments splits a remote's path into the repository's segments: no empty
// ones, no ".git", and no "v3" in front of an Azure ssh path.
func segments(host, path string) []string {
	var out []string
	for _, s := range strings.Split(strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) > 0 && out[0] == "v3" && azureSSH(host) {
		out = out[1:]
	}
	return out
}

func azureSSH(host string) bool {
	return host == "ssh.dev.azure.com" || host == "vs-ssh.visualstudio.com"
}

// redact drops a URL's credentials before it reaches an error: a remote may be
// https://oauth2:<token>@host/..., and an error is written to the log. It hides
// everything up to the last "@", because a token may hold a "/" and a password
// an "@" of its own: over-redacting a path with an "@" in it costs a less
// useful message, and under-redacting costs the credential.
func redact(raw string) string {
	scheme, rest, found := strings.Cut(raw, "://")
	at := strings.LastIndex(rest, "@")
	if !found || at < 0 {
		return raw
	}
	return scheme + "://***@" + rest[at+1:]
}
