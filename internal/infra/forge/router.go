package forge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Router reads each project's forge through the backend its remote names
// (#452). It is the one value cmd builds for everything forge: the cards'
// pull requests, the tracker, an item, the browser and #331's ship calls.
//
//	r := forge.NewRouter(forge.Options{Remote: vcs.NewCLI().RemoteURL, Hosts: cfg.Forge.Hosts})
//	prs, err := r.ListPRs("/p/omatty")
//
// A project's kind is read from its origin once and kept for the run; the tool
// that reads it is found on PATH per call, so installing one mid-run is picked
// up on the next poll.
type Router struct {
	remote    func(repoRoot string) (string, error)
	hosts     Hosts
	transport Transport
	bins      map[Kind]string
	sshBin    string
	lookPath  func(string) (string, error)
	getenv    func(string) string
	rest      restClient
	open      func(url string) error
	ci        *ciCache
	timeout   time.Duration
	azWait    time.Duration // how long az may take to issue a token

	mu        sync.Mutex
	seen      map[string]resolved
	teaLogins map[string]string // remote's instance -> the tea login that reads it, once found
	teaAPIs   map[string]bool   // tea binary -> it has `tea api` (0.12+), once seen
}

// Options is what a Router is built from.
type Options struct {
	// Remote reads a project's origin URL: vcs.CLI.RemoteURL. Injected, so
	// forge does not import vcs and git stays in vcs (invariant 4).
	Remote func(repoRoot string) (string, error)
	// Hosts is the operator's [forge.hosts]; nil is the built-in table alone.
	Hosts Hosts
	// Transport is TransportAuto - CLI first, REST fallback - except in
	// forgeprobe, which reads each transport by name (#463).
	Transport Transport
}

// Transport is which way a Router may read a forge.
type Transport int

const (
	// TransportAuto is the forge's CLI when it is on PATH, else its REST API
	// with a token borrowed from the environment.
	TransportAuto Transport = iota
	// TransportCLI is the CLI alone.
	TransportCLI
	// TransportREST is the REST API alone, even with the CLI installed.
	TransportREST
)

// resolved is what a project's remote says: its forge, and where on it.
type resolved struct {
	kind   Kind
	remote Remote
}

// labels is each readable forge's words. A forge omatty cannot read yet has
// none, so its projects keep the neutral label.
var labels = map[Kind]Label{
	KindGitHub: GitHub, KindGitLab: gitLabLabel, KindGitea: giteaLabel, KindBitbucket: bitbucketLabel,
	KindAzure: azureLabel,
}

// NewRouter builds a Router that runs each forge's own CLI from PATH.
func NewRouter(o Options) *Router {
	return &Router{
		remote: o.Remote, hosts: o.Hosts, transport: o.Transport,
		bins: map[Kind]string{KindGitHub: "gh", KindGitLab: "glab", KindGitea: "tea", KindAzure: "az"}, sshBin: "ssh",
		lookPath: exec.LookPath, getenv: os.Getenv, rest: newREST(), open: openInBrowser,
		ci: newCICache(), timeout: listTimeout, azWait: azTimeout, seen: map[string]resolved{}, teaLogins: map[string]string{}, teaAPIs: map[string]bool{},
	}
}

// ListPRs is the project's open pull requests, then its recently finished ones.
func (r *Router) ListPRs(repoRoot string) ([]PR, error) {
	return call(r, repoRoot, func(ctx context.Context, b backend) ([]PR, error) { return b.listPRs(ctx, repoRoot) })
}

// ListIssues is the project's open issues.
func (r *Router) ListIssues(repoRoot string) ([]Issue, error) {
	return call(r, repoRoot, func(ctx context.Context, b backend) ([]Issue, error) { return b.listIssues(ctx, repoRoot) })
}

// ViewIssue is one issue in full.
func (r *Router) ViewIssue(repoRoot string, number int) (Detail, error) {
	return call(r, repoRoot, func(ctx context.Context, b backend) (Detail, error) {
		return b.viewIssue(ctx, repoRoot, number)
	})
}

// ViewPR is one pull request in full.
func (r *Router) ViewPR(repoRoot string, number int) (Detail, error) {
	return call(r, repoRoot, func(ctx context.Context, b backend) (Detail, error) {
		return b.viewPR(ctx, repoRoot, number)
	})
}

// BrowseIssue opens an issue in the operator's browser.
func (r *Router) BrowseIssue(repoRoot string, number int) error {
	return r.browse(repoRoot, number, false)
}

// BrowsePR opens a pull request in the operator's browser. Separate from
// BrowseIssue because GitLab, Azure and Bitbucket number the two apart.
func (r *Router) BrowsePR(repoRoot string, number int) error { return r.browse(repoRoot, number, true) }

func (r *Router) browse(repoRoot string, number int, pr bool) error {
	_, err := call(r, repoRoot, func(ctx context.Context, b backend) (struct{}, error) {
		return struct{}{}, b.browse(ctx, repoRoot, number, pr)
	})
	return err
}

// Label is the project's forge words, from what the Router has already
// resolved: it is called while the window draws, so it never reads the remote
// or runs anything. Neutral until a call has resolved the project.
func (r *Router) Label(repoRoot string) Label {
	res, ok := r.cached(repoRoot)
	if !ok {
		return Neutral
	}
	return labelOf(res)
}

// labelOf is a resolved project's words: its kind's, and for a Gitea host the
// name the operator knows it by.
func labelOf(res resolved) Label {
	if res.kind == KindGitea && res.remote.Host == "codeberg.org" {
		return codebergLabel
	}
	if l, readable := labels[res.kind]; readable {
		return l
	}
	return Neutral
}

// call is every Router method's shape: resolve the project's backend, then give
// the call its one bounded context.
func call[T any](r *Router, repoRoot string, do func(context.Context, backend) (T, error)) (T, error) {
	b, err := r.backendFor(repoRoot)
	if err != nil {
		var zero T
		return zero, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	return do(ctx, b)
}

// backendFor is the backend that reads repoRoot's forge now.
func (r *Router) backendFor(repoRoot string) (backend, error) {
	res, err := r.resolve(repoRoot)
	if err != nil {
		return nil, err
	}
	return r.pick(repoRoot, res)
}

// pick chooses the backend for a resolved project. A forge whose backend has
// not landed is ErrNoForge, never another forge's CLI asked about a repository
// it cannot see.
func (r *Router) pick(repoRoot string, res resolved) (backend, error) {
	switch res.kind {
	case KindGitHub:
		return r.pickGitHub(res.remote)
	case KindGitLab:
		return r.pickGitLab(repoRoot, res.remote)
	case KindGitea:
		return r.pickGitea(repoRoot, res.remote)
	case KindBitbucket:
		return r.pickBitbucket(res.remote)
	case KindAzure:
		return r.pickAzure(res.remote)
	}
	return nil, fmt.Errorf("forge: omatty does not read %s yet: %w", res.kind, ErrNoForge)
}

// pickGitLab is glab when it is installed (#454), else GitLab's REST API with
// a token glab itself reads (#455) - the same paths either way, so the same
// fold.
func (r *Router) pickGitLab(repoRoot string, remote Remote) (backend, error) {
	var f fetcher
	if bin, ok := r.cli(KindGitLab); ok {
		// The bare host: glab refuses a --hostname with a port ("invalid
		// hostname") and takes the API's port from its own per-host config.
		f = cliAPI{bin: bin, host: remote.Host, dir: repoRoot, timeout: r.timeout}
	} else if remote.Scheme == "http" {
		// A token never crosses plain http, so only glab, which keeps its own
		// per-host protocol, can read an http remote: a note, not a request
		// that fails every poll (#584).
		return nil, &MissingToolError{Tool: "glab"}
	} else if tok, env := r.borrow([]string{"GITLAB_TOKEN", "GITLAB_ACCESS_TOKEN"}); tok != "" && r.transport != TransportCLI {
		f = restAPI{rest: r.rest, base: webBase(remote) + "/api/v4", auth: privateToken(tok), env: env}
	} else {
		return nil, &MissingToolError{Tool: "glab", TokenEnv: "GITLAB_TOKEN"}
	}
	return glBackend{f: f, remote: remote, ci: r.ci, open: r.open}, nil
}

// pickGitHub is gh when it is installed, else GitHub's HTTP API with a token
// gh itself would read, else the note naming both halves of the fix (#462).
func (r *Router) pickGitHub(remote Remote) (backend, error) {
	if bin, ok := r.cli(KindGitHub); ok {
		return ghCLI{bin: bin, timeout: r.timeout}, nil
	}
	envs := gitHubTokens(remote.Host)
	if tok, env := r.borrow(envs); tok != "" && r.transport != TransportCLI {
		return ghHTTP{rest: r.rest, auth: bearer(tok), env: env, remote: remote, open: r.open}, nil
	}
	return nil, &MissingToolError{Tool: "gh", TokenEnv: envs[0]}
}

// cli is kind's CLI, when this Router may run it and it is on PATH.
func (r *Router) cli(kind Kind) (string, bool) {
	if r.transport == TransportREST {
		return "", false
	}
	bin := r.bins[kind]
	_, err := r.lookPath(bin)
	return bin, err == nil
}

// borrow is the first of envs that is set, and its name. Read on every call
// and kept by nothing: omatty stores no token (#453).
func (r *Router) borrow(envs []string) (string, string) {
	for _, env := range envs {
		if tok := r.getenv(env); tok != "" {
			return tok, env
		}
	}
	return "", ""
}
