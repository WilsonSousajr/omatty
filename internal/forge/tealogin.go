package forge

import (
	"encoding/json"
	"net/url"
	"strings"
)

// pickGitea is tea, through the login it holds for the project's host (#458),
// else Gitea's REST API (#459): with GITEA_TOKEN when it is set - the
// variable tea's own env login reads - and anonymously when it is not, which
// is enough for a public repository on Codeberg. tea cannot read anonymously
// and has no --hostname, so a tea without a login for this host is passed over.
func (r *Router) pickGitea(repoRoot string, remote Remote) (backend, error) {
	if bin, ok := r.cli(KindGitea); ok {
		// A tea older than 0.12 has no `tea api` and is passed over too (#458).
		if login := r.teaLogin(bin, remote); login != "" && r.teaHasAPI(bin) {
			f := teaAPI{bin: bin, login: login, host: apiHost(remote), dir: repoRoot, timeout: r.timeout}
			return gtBackend{f: f, remote: remote, ci: r.ci, open: r.open}, nil
		}
	}
	if r.transport == TransportCLI {
		return nil, &MissingToolError{Tool: "tea", TokenEnv: "GITEA_TOKEN"}
	}
	tok, _ := r.borrow([]string{"GITEA_TOKEN"})
	a := anonymous()
	if tok != "" {
		a = tokenAuth(tok)
	}
	f := restAPI{rest: r.rest, base: webBase(remote) + "/api/v1", auth: a, env: "GITEA_TOKEN"}
	return gtBackend{f: f, remote: remote, ci: r.ci, open: r.open, anon: tok == ""}, nil
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
// two write it (#458's review): hosts in any case; an ssh remote by the
// login's host alone, since its port is ssh's, or by the login's own ssh
// host; an http(s) remote by host and port, a scheme's default port written
// or not.
func (l teaLoginEntry) serves(remote Remote) bool {
	u, err := url.Parse(l.URL)
	if err != nil {
		return false
	}
	if remote.Scheme == "ssh" {
		return strings.EqualFold(u.Hostname(), remote.Host) || strings.EqualFold(l.SSHHost, remote.Host)
	}
	return strings.EqualFold(u.Hostname(), remote.Host) && webPort(u.Scheme, u.Port()) == webPort(remote.Scheme, remote.Port)
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
