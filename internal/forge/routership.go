package forge

import "context"

// CreatePR opens a pull request for head against base and returns its number
// (#331).
func (r *Router) CreatePR(repoRoot, head, base, title string) (int, error) {
	return call(r, repoRoot, func(ctx context.Context, b backend) (int, error) {
		return b.createPR(ctx, repoRoot, head, base, title)
	})
}

// MergePR merges a pull request with the repository's own method (#331), at
// head - the commit that was green, so a push since is never merged unread
// (#599) - and reports whether it is merged now rather than accepted.
func (r *Router) MergePR(repoRoot string, number int, head string) (bool, error) {
	return call(r, repoRoot, func(ctx context.Context, b backend) (bool, error) {
		return b.mergePR(ctx, repoRoot, number, head)
	})
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
