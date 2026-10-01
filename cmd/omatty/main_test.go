package main

import (
	"context"
	"path/filepath"
	"testing"

	statestore "github.com/WilsonSousajr/omatty/internal/infra/store"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// cmd/omatty had no test file at all, and the coverage gate measures only
// ./internal/..., so every line here was unmeasured as well as unexercised -
// which is why the readLine EOF path and the projectRegistrar name mismatch
// both shipped (#91). These cover the parts that hold logic; the wiring in
// tuiDeps and run is exercised by the milestone's PTY smoke test, which a
// person reads (AGENTS.md, "Build and test commands").

// FakeGit is a named fake for the git methods the adapters here need.
//
// RepoRoot and MainCheckout answer from separate maps, and that separation is
// the point: they are different questions - inside a linked worktree the first
// returns the worktree and the second the repository it was forked from - and
// one shared answer for both is what made adoption's worktree bug invisible to
// this package (#91, #122). Worktrees fills MainCheckout alone.
type FakeGit struct {
	Roots     map[string]string
	Worktrees map[string]string
	Branch    string
}

func (f *FakeGit) RepoRoot(_ context.Context, dir string) (string, error) {
	return lookup(f.Roots, dir)
}

func (f *FakeGit) MainCheckout(dir string) (string, error) {
	if root, ok := f.Worktrees[dir]; ok {
		return root, nil
	}
	return lookup(f.Roots, dir)
}

func (f *FakeGit) CurrentBranch(context.Context, string) (string, error) { return f.Branch, nil }

func (f *FakeGit) RemoveWorktree(string, string) error { return nil }

func lookup(roots map[string]string, dir string) (string, error) {
	if root, ok := roots[dir]; ok {
		return root, nil
	}
	return "", errNotARepo{dir}
}

type errNotARepo struct{ dir string }

func (e errNotARepo) Error() string { return "not a git repository: " + e.dir }

// storeIn builds a registry over a temporary state.json.
func storeIn(t *testing.T) sessions.StateStore {
	t.Helper()
	return statestore.NewStore(filepath.Join(t.TempDir(), "state.json"))
}
