package forge

import (
	"slices"
	"strings"

	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
)

// Resolve names r's forge, and for a forge served under a root moves the
// root's segments off r.Path into r.Root, so the slug is the repository's
// alone (#585). An http(s) remote takes the key whose root is the longest
// prefix of its path and still leaves a repository. An ssh remote carries no
// root, so its host's bare key wins, else its one rooted key; with several,
// it is refused aloud rather than guessed. Anything else is KindOf's answer.
//
//	forge.Hosts{"h/gitlab": forge.KindGitLab}.Resolve(remote) // Root ["gitlab"]
func (h Hosts) Resolve(r Remote) (Remote, Kind, error) {
	roots := h.rootsOn(r.Host)
	if r.Scheme != "ssh" {
		if key, root, ok := longestRoot(roots, r.Path); ok {
			r.Root, r.Path = root, r.Path[len(root):]
			return r, h[key], nil
		}
	} else if len(roots) > 0 && !h.hasBare(r.Host) {
		return h.sshRoot(r, roots)
	}
	kind, err := h.KindOf(r.Host)
	return r, kind, err
}

// rootsOn is each rooted key on host, by key, with its root.
func (h Hosts) rootsOn(host string) map[string][]string {
	roots := map[string][]string{}
	for key := range h {
		if keyHost, root := SplitHostKey(key); root != nil && strings.EqualFold(keyHost, host) {
			roots[key] = root
		}
	}
	return roots
}

// hasBare is whether a key names host with no root.
func (h Hosts) hasBare(host string) bool {
	for key := range h {
		if strings.EqualFold(key, host) {
			return true
		}
	}
	return false
}

// longestRoot is the key whose root is the longest segment prefix of path
// that still leaves a repository. A root is the server's path: compared as
// written, case and all.
func longestRoot(roots map[string][]string, path []string) (string, []string, bool) {
	best, bestRoot := "", []string(nil)
	for key, root := range roots {
		fits := len(root) < len(path) && slices.Equal(path[:len(root)], root)
		if fits && len(root) > len(bestRoot) {
			best, bestRoot = key, root
		}
	}
	return best, bestRoot, best != ""
}

// sshRoot is an ssh remote on a host whose only keys are rooted: the one
// root, or a refusal when there are several.
func (h Hosts) sshRoot(r Remote, roots map[string][]string) (Remote, Kind, error) {
	if len(roots) > 1 {
		return r, "", &dforge.AmbiguousHostError{Host: r.Host}
	}
	for key, root := range roots {
		r.Root = root
		return r, h[key], nil
	}
	return r, "", nil
}
