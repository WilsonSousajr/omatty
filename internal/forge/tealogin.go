package forge

import (
	"encoding/json"
	"net/url"
)

// pickGitea is tea, through the login it holds for the project's host (#458).
// tea cannot read anonymously and has no --hostname, so a tea without a login
// for this host is no way in; the REST fallback is #459.
func (r *Router) pickGitea(repoRoot string, remote Remote) (backend, error) {
	bin, ok := r.cli(KindGitea)
	if !ok {
		return nil, &MissingToolError{Tool: "tea"}
	}
	login := r.teaLogin(bin, apiHost(remote))
	if login == "" {
		// Installed, and still no way in: not "tea is not installed" (#586).
		return nil, &MissingToolError{Tool: "tea", NoLoginFor: apiHost(remote)}
	}
	f := teaAPI{bin: bin, login: login, host: apiHost(remote), dir: repoRoot, timeout: r.timeout}
	return gtBackend{f: f, remote: remote, ci: r.ci, open: r.open}, nil
}

// teaLogin is the name of the tea login whose URL is host, read from
// `tea logins list`, which is tea's own config and touches no network. Kept
// once found; a host with none is asked again, so a login added mid-run is
// used on the next poll.
func (r *Router) teaLogin(bin, host string) string {
	r.mu.Lock()
	name, known := r.teaLogins[host]
	r.mu.Unlock()
	if known {
		return name
	}
	out, err := r.lookup(bin, "logins", "list", "--output", "json")
	if name = loginFor(out, host); err != nil || name == "" {
		return ""
	}
	r.mu.Lock()
	r.teaLogins[host] = name
	r.mu.Unlock()
	return name
}

// loginFor is the login in tea's list whose URL names host, or "".
func loginFor(list []byte, host string) string {
	var logins []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if json.Unmarshal(list, &logins) != nil {
		return ""
	}
	for _, l := range logins {
		if u, err := url.Parse(l.URL); err == nil && u.Host == host {
			return l.Name
		}
	}
	return ""
}

// apiHost is the host and, for an http(s) remote on one, the port: how a tea
// login's URL names the instance it reads.
func apiHost(r Remote) string {
	if r.Scheme == "ssh" || r.Port == "" {
		return r.Host
	}
	return r.Host + ":" + r.Port
}
