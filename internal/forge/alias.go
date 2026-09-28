package forge

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// aliasTimeout bounds ssh -G, which reads local configuration and never
// connects: a second is generous, and the poll must not wait on it.
const aliasTimeout = 2 * time.Second

// safeAlias is an ssh Host alias that cannot be read as an option: a host from
// a remote's URL is handed to ssh as an argument, and "-oProxyCommand=..." is a
// valid scp-style host to git.
var safeAlias = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// name is remote's forge, and the remote as that forge sees it. The operator's
// table and the built-in one answer first. An ssh remote neither knows may be
// on an ssh Host alias - git@github-work:acme/app, how people with two GitHub
// accounts clone - so its real host is asked of ssh, the way gh resolves it
// (#576). An https host is a DNS name and never an alias, so it is never asked.
func (r *Router) name(remote Remote) (Remote, Kind, error) {
	kind, err := r.hosts.KindOf(remote.Host)
	if err == nil || remote.Scheme != "ssh" || !safeAlias.MatchString(remote.Host) {
		return remote, kind, err
	}
	host, lookErr := r.sshHostname(remote.Host)
	if lookErr != nil || strings.EqualFold(host, remote.Host) {
		return remote, "", err
	}
	remote.Host = strings.ToLower(host)
	kind, err = r.hosts.KindOf(remote.Host)
	return remote, kind, err
}

// sshHostname is the host ssh would connect to for alias, read off `ssh -G`,
// which evaluates ~/.ssh/config and prints the result without connecting.
func (r *Router) sshHostname(alias string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), aliasTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, r.sshBin, "-G", "--", alias).Output()
	if err != nil {
		return "", fmt.Errorf("forge: ssh -G %s: %w", alias, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if host, found := strings.CutPrefix(line, "hostname "); found {
			return strings.TrimSpace(host), nil
		}
	}
	return "", fmt.Errorf("forge: ssh -G %s printed no hostname", alias)
}
