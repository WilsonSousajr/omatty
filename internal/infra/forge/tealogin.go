package forge

import (
	"encoding/json"
	"net/url"
	"strings"
)

// pickGitea is tea, through the login it holds for the project's host (#458),
// else Gitea's REST API (#459). tea cannot read anonymously and has no
// --hostname, so a tea without a login for this host is passed over - and
// named as such in any note, since it is installed (#586).
func (r *Router) pickGitea(repoRoot string, remote Remote) (backend, error) {
	// The token is only ever this instance's, so the note says which (#589).
	missing := &MissingToolError{Tool: "tea", TokenEnv: "GITEA_TOKEN for " + webBase(remote)}
	if bin, installed := r.cli(KindGitea); installed {
		if b := r.teaBackend(bin, repoRoot, remote, missing); b != nil {
			return b, nil
		}
	}
	if r.transport == TransportCLI {
		return nil, missing
	}
	return r.giteaREST(remote, missing)
}

// teaBackend is tea reading remote through its login, or nil with missing
// saying why not: no login for the host (#586), or a tea older than 0.12,
// which has no `tea api` (#458).
func (r *Router) teaBackend(bin, repoRoot string, remote Remote, missing *MissingToolError) backend {
	login := r.teaLogin(bin, remote)
	switch {
	case login == "":
		missing.NoLoginFor = apiHost(remote)
	case !r.teaHasAPI(bin):
		missing.Tool = "tea 0.12 or later"
	default:
		f := teaAPI{bin: bin, login: login, host: apiHost(remote), dir: repoRoot, timeout: r.timeout}
		return gtBackend{f: f, remote: remote, ci: r.ci, open: r.open}
	}
	return nil
}

// giteaREST reads with a token for this instance when there is one, and
// anonymously when there is not, which is enough for a public repository on
// Codeberg; missing is what an anonymous refusal says. A token never crosses
// plain http (#584), so an http remote is read anonymously whatever is set,
// and its note names tea alone, since no token could help.
func (r *Router) giteaREST(remote Remote, missing *MissingToolError) (backend, error) {
	tok := r.giteaToken(remote)
	if remote.Scheme == "http" {
		tok, missing.TokenEnv = "", ""
	}
	if tok == "" {
		f := restAPI{rest: r.rest, base: webBase(remote) + "/api/v1", auth: anonymous(), env: "GITEA_TOKEN"}
		return gtBackend{f: f, remote: remote, ci: r.ci, open: r.open, needsToken: missing}, nil
	}
	f := restAPI{rest: r.rest, base: webBase(remote) + "/api/v1", auth: tokenAuth(tok), env: "GITEA_TOKEN"}
	return gtBackend{f: f, remote: remote, ci: r.ci, open: r.open}, nil
}

// giteaToken is GITEA_TOKEN when GITEA_INSTANCE_URL names remote's instance:
// tea's own env login, which binds the token to one instance. A token set for
// another is never sent here - #589's review found a corporate token going to
// Codeberg - and one set with no instance at all is not sent anywhere.
func (r *Router) giteaToken(remote Remote) string {
	tok, _ := r.borrow([]string{"GITEA_TOKEN"})
	instance, _ := r.borrow([]string{"GITEA_INSTANCE_URL"})
	if tok == "" || !(teaLoginEntry{URL: instance}).serves(remote) {
		return ""
	}
	return tok
}

// teaLogin is the name of the tea login for the instance remote is on, read
// from `tea logins list`, which is tea's own config and touches no network.
// Kept once found; a remote with none is asked again, so a login added mid-run
// is used on the next poll.
func (r *Router) teaLogin(bin string, remote Remote) string {
	key := remote.Scheme + "://" + apiHost(remote)
	r.mu.Lock()
	name, known := r.teaLogins[key]
	r.mu.Unlock()
	if known {
		return name
	}
	out, err := r.lookup(bin, "logins", "list", "--output", "json")
	if name = loginFor(out, remote); err != nil || name == "" {
		return ""
	}
	r.mu.Lock()
	r.teaLogins[key] = name
	r.mu.Unlock()
	return name
}

// teaHasAPI is whether bin has `tea api`, by the exit status of its help:
// kept once it does, asked again while it does not, as a login is.
func (r *Router) teaHasAPI(bin string) bool {
	r.mu.Lock()
	known := r.teaAPIs[bin]
	r.mu.Unlock()
	if known {
		return true
	}
	if _, err := r.lookup(bin, "api", "--help"); err != nil {
		return false
	}
	r.mu.Lock()
	r.teaAPIs[bin] = true
	r.mu.Unlock()
	return true
}

// teaLoginEntry is one login in `tea logins list --output json`.
type teaLoginEntry struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	SSHHost string `json:"ssh_host"`
}

// loginFor is the login in tea's list for the instance remote names, or "".
func loginFor(list []byte, remote Remote) string {
	var logins []teaLoginEntry
	if json.Unmarshal(list, &logins) != nil {
		return ""
	}
	for _, l := range logins {
		if l.serves(remote) {
			return l.Name
		}
	}
	return ""
}

// serves is whether the login reads the instance remote is on, however the
// two write it (#458's review): by its URL, or for an ssh remote by the
// login's own ssh host.
func (l teaLoginEntry) serves(remote Remote) bool {
	return namesInstance(l.URL, remote) || remote.Scheme == "ssh" && strings.EqualFold(l.SSHHost, remote.Host)
}

// namesInstance is whether rawURL is the instance remote is on: hosts in any
// case; an ssh remote by host alone, since its port is ssh's; an http(s)
// remote by host and port, a scheme's default port written or not. A tea
// login (#458) and BITBUCKET_DC_URL (#461) are matched the same way.
func namesInstance(rawURL string, remote Remote) bool {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Hostname(), remote.Host) {
		return false
	}
	return remote.Scheme == "ssh" || webPort(u.Scheme, u.Port()) == webPort(remote.Scheme, remote.Port)
}

// webPort is a port with a scheme's default written in.
func webPort(scheme, port string) string {
	switch {
	case port != "":
		return port
	case scheme == "http":
		return "80"
	}
	return "443"
}

// apiHost is the host and, for an http(s) remote on one, the port: how a tea
// login's URL names the instance it reads.
func apiHost(r Remote) string {
	if r.Scheme == "ssh" || r.Port == "" {
		return r.Host
	}
	return r.Host + ":" + r.Port
}
