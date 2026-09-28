package forge

import (
	"context"
	"fmt"
)

// backend reads one forge for the Router, over one transport. Unexported: the
// UI depends on func types it declares itself, and a forge is a backend here,
// never an interface another package implements (ARCHITECTURE.md's seam rule).
type backend interface {
	listPRs(ctx context.Context, repoRoot string) ([]PR, error)
	listIssues(ctx context.Context, repoRoot string) ([]Issue, error)
	viewIssue(ctx context.Context, repoRoot string, number int) (Detail, error)
	viewPR(ctx context.Context, repoRoot string, number int) (Detail, error)
	browse(ctx context.Context, repoRoot string, number int, pr bool) error
}

// shipper is a backend that can carry #331's three actions. Separate from
// backend because not every transport has them yet (#464), and a backend
// without them is refused by name rather than given stubs that pretend.
type shipper interface {
	createPR(ctx context.Context, repoRoot, head, base, title string) (int, error)
	mergePR(ctx context.Context, repoRoot string, number int) error
	branchProtected(ctx context.Context, repoRoot, branch string) (bool, error)
}

// resolve is repoRoot's forge, read once and kept. A failure is not kept, so a
// remote added after omatty started is read on the next call. Read outside the
// lock: two callers racing to read the same remote cost one extra git call,
// while a lock held across it would stall every other project behind one slow
// repository.
func (r *Router) resolve(repoRoot string) (resolved, error) {
	if res, ok := r.cached(repoRoot); ok {
		return res, nil
	}
	res, err := r.lookUp(repoRoot)
	if err != nil {
		return resolved{}, err
	}
	r.mu.Lock()
	r.seen[repoRoot] = res
	r.mu.Unlock()
	return res, nil
}

func (r *Router) cached(repoRoot string) (resolved, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.seen[repoRoot]
	return res, ok
}

// lookUp reads repoRoot's origin and names its forge. Every way of failing is
// ErrNoForge: no origin, one that is not a repository URL, a host omatty does
// not know. The project simply has nothing to show.
func (r *Router) lookUp(repoRoot string) (resolved, error) {
	raw, err := r.remote(repoRoot)
	if err != nil {
		return resolved{}, fmt.Errorf("forge: %q has no origin to read: %v: %w", repoRoot, err, ErrNoForge)
	}
	remote, err := ParseRemote(raw)
	if err != nil {
		return resolved{}, fmt.Errorf("%v: %w", err, ErrNoForge)
	}
	remote, kind, err := r.name(remote)
	if err != nil {
		return resolved{}, err
	}
	return resolved{kind: kind, remote: webHost(remote)}, nil
}
