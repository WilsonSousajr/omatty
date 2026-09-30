package vcs

import "context"

// Contextual is the CLI behind ports whose methods take a context (ADR 0001:
// every port method that does I/O takes one; migration step 5.4, #653). The
// caller's context becomes the parent of each git call, so cancelling it ends
// a git still running; the call's own deadline still bounds it.
//
//	git := vcs.NewCLI().Contextual()
//	root, err := git.RepoRoot(ctx, dir)
type Contextual struct{ cli *CLI }

// Contextual returns c behind context-taking methods.
func (c *CLI) Contextual() Contextual { return Contextual{cli: c} }

// under is the CLI with ctx as the parent of every call it makes.
func (g Contextual) under(ctx context.Context) *CLI {
	cp := *g.cli
	cp.parent = ctx
	return &cp
}

// parentOrBackground is the context a call's deadline is derived from.
func (c *CLI) parentOrBackground() context.Context {
	if c.parent == nil {
		return context.Background()
	}
	return c.parent
}

// RepoRoot is CLI.RepoRoot under ctx.
func (g Contextual) RepoRoot(ctx context.Context, dir string) (string, error) {
	return g.under(ctx).RepoRoot(dir)
}

// CurrentBranch is CLI.CurrentBranch under ctx.
func (g Contextual) CurrentBranch(ctx context.Context, dir string) (string, error) {
	return g.under(ctx).CurrentBranch(dir)
}

// MainCheckout is CLI.MainCheckout under ctx.
func (g Contextual) MainCheckout(ctx context.Context, dir string) (string, error) {
	return g.under(ctx).MainCheckout(dir)
}

// CommitsOnBranch is CLI.CommitsOnBranch under ctx.
func (g Contextual) CommitsOnBranch(ctx context.Context, repoRoot, base, branch string) (int, error) {
	return g.under(ctx).CommitsOnBranch(repoRoot, base, branch)
}

// RenameBranch is CLI.RenameBranch under ctx.
func (g Contextual) RenameBranch(ctx context.Context, repoRoot, old, name string) error {
	return g.under(ctx).RenameBranch(repoRoot, old, name)
}

// AddWorktree is CLI.AddWorktree under ctx.
func (g Contextual) AddWorktree(ctx context.Context, repoRoot, dir, branch, base string) error {
	return g.under(ctx).AddWorktree(repoRoot, dir, branch, base)
}

// RemoveWorktree is CLI.RemoveWorktree under ctx.
func (g Contextual) RemoveWorktree(ctx context.Context, repoRoot, dir string) error {
	return g.under(ctx).RemoveWorktree(repoRoot, dir)
}
