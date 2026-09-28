package forge

import (
	"context"
	"fmt"
)

// CreatePR opens a pull request for head against base and returns its number
// (#331).
func (r *Router) CreatePR(repoRoot, head, base, title string) (int, error) {
	return call(r, repoRoot, func(ctx context.Context, b backend) (int, error) {
		s, err := shipperOf(b)
		if err != nil {
			return 0, err
		}
		return s.createPR(ctx, repoRoot, head, base, title)
	})
}

// MergePR merges a pull request with the repository's own method (#331).
func (r *Router) MergePR(repoRoot string, number int) error {
	_, err := call(r, repoRoot, func(ctx context.Context, b backend) (struct{}, error) {
		s, err := shipperOf(b)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, s.mergePR(ctx, repoRoot, number)
	})
	return err
}

// BranchProtected reports whether branch is protected on the project's forge.
// It fails closed: every error, the Router's own included, comes back beside
// true, so a caller reading the bool alone still refuses (#331).
func (r *Router) BranchProtected(repoRoot, branch string) (bool, error) {
	protected, err := call(r, repoRoot, func(ctx context.Context, b backend) (bool, error) {
		s, err := shipperOf(b)
		if err != nil {
			return true, err
		}
		return s.branchProtected(ctx, repoRoot, branch)
	})
	if err != nil {
		return true, err
	}
	return protected, nil
}

// shipperOf is b's ship actions, or a refusal saying they are not built for
// this transport yet: a stub that pretended would be worse (#464).
func shipperOf(b backend) (shipper, error) {
	if s, ok := b.(shipper); ok {
		return s, nil
	}
	return nil, fmt.Errorf("forge: omatty cannot ship through this forge's %T yet; use the forge's own tools", b)
}
