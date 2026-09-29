package forge

import "context"

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
