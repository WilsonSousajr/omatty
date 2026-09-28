package forge

import (
	"context"
	"fmt"
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
	remote   func(repoRoot string) (string, error)
	hosts    Hosts
	bins     map[Kind]string
	sshBin   string
	lookPath func(string) (string, error)
	timeout  time.Duration

	mu   sync.Mutex
	seen map[string]resolved
}

// Options is what a Router is built from.
type Options struct {
	// Remote reads a project's origin URL: vcs.CLI.RemoteURL. Injected, so
	// forge does not import vcs and git stays in vcs (invariant 4).
	Remote func(repoRoot string) (string, error)
	// Hosts is the operator's [forge.hosts]; nil is the built-in table alone.
	Hosts Hosts
}

// resolved is what a project's remote says: its forge, and where on it.
type resolved struct {
	kind   Kind
	remote Remote
}

// labels is each readable forge's words. A forge omatty cannot read yet has
// none, so its projects keep the neutral label.
var labels = map[Kind]Label{KindGitHub: GitHub}

// NewRouter builds a Router that runs each forge's own CLI from PATH.
func NewRouter(o Options) *Router {
	return &Router{
		remote: o.Remote, hosts: o.Hosts,
		bins: map[Kind]string{KindGitHub: "gh"}, sshBin: "ssh",
		lookPath: exec.LookPath, timeout: listTimeout,
		seen: map[string]resolved{},
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
	return r.pick(res)
}

// pick chooses the backend for a resolved project. Only GitHub is read so far;
// another forge is ErrNoForge until its backend lands, never gh asked about a
// repository it cannot see.
func (r *Router) pick(res resolved) (backend, error) {
	if res.kind != KindGitHub {
		return nil, fmt.Errorf("forge: omatty does not read %s yet: %w", res.kind, ErrNoForge)
	}
	bin := r.bins[KindGitHub]
	if _, err := r.lookPath(bin); err != nil {
		return nil, &MissingToolError{Tool: "gh"}
	}
	return ghCLI{bin: bin, timeout: r.timeout}, nil
}

// CreatePR opens a pull request for head against base and returns its number
// (#331).
func (r *Router) CreatePR(repoRoot, head, base, title string) (int, error) {
	return call(r, repoRoot, func(ctx context.Context, b backend) (int, error) {
		return b.createPR(ctx, repoRoot, head, base, title)
	})
}

// MergePR merges a pull request with the repository's own method (#331).
func (r *Router) MergePR(repoRoot string, number int) error {
	_, err := call(r, repoRoot, func(ctx context.Context, b backend) (struct{}, error) {
		return struct{}{}, b.mergePR(ctx, repoRoot, number)
	})
	return err
}

// BranchProtected reports whether branch is protected on the project's forge.
// It fails closed: every error, the Router's own included, comes back beside
// true, so a caller reading the bool alone still refuses (#331).
func (r *Router) BranchProtected(repoRoot, branch string) (bool, error) {
	protected, err := call(r, repoRoot, func(ctx context.Context, b backend) (bool, error) {
		return b.branchProtected(ctx, repoRoot, branch)
	})
	if err != nil {
		return true, err
	}
	return protected, nil
}
