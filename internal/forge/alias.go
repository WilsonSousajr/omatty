package forge

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// aliasTimeout bounds each local lookup - ssh -G, gh auth token - which read
// configuration and never connect: two seconds is generous, and the poll must
// not wait on them.
const aliasTimeout = 2 * time.Second

// safeAlias is a host that cannot be read as an option: a host from a remote's
// URL is handed to ssh and gh as an argument, and "-oProxyCommand=..." is a
// valid scp-style host to git.
var safeAlias = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// name is remote's forge, and the remote as that forge sees it. The operator's
// table and the built-in one answer first. Then two local lookups, the ways gh
// itself would have found the host before #452 named forges by host:
//   - an ssh remote may be on a Host alias - git@github-work:acme/app, how
//     people with two GitHub accounts clone - resolved by ssh -G (#576);
//   - a host gh holds a login for is a GitHub Enterprise host (#579).
//
// Neither touches the network, and a host that could read as an option is
// handed to neither.
func (r *Router) name(remote Remote) (Remote, Kind, error) {
	kind, err := r.hosts.KindOf(remote.Host)
	if err == nil || !safeAlias.MatchString(remote.Host) {
		return remote, kind, err
	}
	remote = r.unalias(remote)
	if aliased, aliasErr := r.hosts.KindOf(remote.Host); aliasErr == nil {
		return remote, aliased, nil
	}
	if r.ghKnows(remote.Host) {
		return remote, KindGitHub, nil
	}
	return remote, "", err
}

// unalias is an ssh remote with its Host alias replaced by the host it stands
// for; any other remote, or an alias ssh cannot resolve, comes back unchanged.
func (r *Router) unalias(remote Remote) Remote {
	if remote.Scheme != "ssh" {
		return remote
	}
	host, err := r.sshHostname(remote.Host)
	if err != nil || !safeAlias.MatchString(host) {
		return remote
	}
	remote.Host = strings.ToLower(host)
	return remote
}

// sshHostname is the host ssh would connect to for alias, read off `ssh -G`,
// which evaluates ~/.ssh/config and prints the result without connecting.
func (r *Router) sshHostname(alias string) (string, error) {
	out, err := r.lookup(r.sshBin, "-G", "--", alias)
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

// ghKnows is whether gh holds a login for host: `gh auth token --hostname`,
// which reads gh's own config and keyring and touches no network. What it
// prints - the token - is discarded unread; only its exit status is used.
func (r *Router) ghKnows(host string) bool {
	bin := r.bins[KindGitHub]
	if _, err := r.lookPath(bin); err != nil {
		return false
	}
	_, err := r.lookup(bin, "auth", "token", "--hostname", host)
	return err == nil
}

// lookup runs one local lookup inside aliasTimeout. Its stderr is discarded
// and WaitDelay set, so a child it spawns - ssh runs a config's Match exec -
// cannot hold it past the bound (#356's trap).
func (r *Router) lookup(bin string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), aliasTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stderr, cmd.WaitDelay = io.Discard, time.Second
	return cmd.Output()
}
