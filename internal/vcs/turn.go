package vcs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// turnRef names a session's turn baseline. Refs live in the common
// repository, so a worktree session's baseline is visible from the main
// checkout and outlives omatty (#311). The id is the session's row id, which
// never changes, so state.json needs no field (invariant 9).
func turnRef(id string) string { return "refs/omatty/turn/" + id }

// SnapshotTree writes dir's working tree as a tree object and returns its id.
// It runs `add -A` and `write-tree` against a temporary copy of the index, so
// the operator's staging, HEAD and stash are never touched and no git
// identity is needed, since no commit is made (#311).
//
//	tree, err := vcs.NewCLI().SnapshotTree("/wt/parser-fix")
func (c *CLI) SnapshotTree(dir string) (string, error) {
	tmp, err := os.MkdirTemp("", "omatty-index-")
	if err != nil {
		return "", fmt.Errorf("vcs: temporary index for %q: %w", dir, err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	index := filepath.Join(tmp, "index")
	if err := c.seedIndex(dir, index); err != nil {
		return "", err
	}
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := c.captureEnv(dir, 0, env, "add", "-A"); err != nil {
		return "", err
	}
	out, err := c.captureEnv(dir, 0, env, "write-tree")
	return strings.TrimSpace(out), err
}

// seedIndex copies dir's own index to path, so `add -A` reuses git's stat
// cache instead of hashing every file. `--git-path` answers relative to dir
// in a main checkout and absolutely in a linked worktree. A repository with
// no index yet leaves path absent, and git starts from empty.
func (c *CLI) seedIndex(dir, path string) error {
	src, err := c.run(dir, "rev-parse", "--git-path", "index")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(dir, src)
	}
	body, err := os.ReadFile(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("vcs: reading the index of %q: %w", dir, err)
	}
	return os.WriteFile(path, body, 0o600)
}

// SetTurnRef points id's turn baseline at tree.
func (c *CLI) SetTurnRef(dir, id, tree string) error {
	_, err := c.run(dir, "update-ref", turnRef(id), tree)
	return err
}

// TurnRef is the tree id's baseline names; false when there is none, which
// rev-parse --verify --quiet reports as exit 1 with no output.
func (c *CLI) TurnRef(dir, id string) (string, bool, error) {
	out, err := c.capture(dir, 1, "rev-parse", "--verify", "--quiet", turnRef(id))
	tree := strings.TrimSpace(out)
	return tree, err == nil && tree != "", err
}

// DeleteTurnRef removes id's baseline. git succeeds on a ref that is not
// there, which archive relies on.
func (c *CLI) DeleteTurnRef(dir, id string) error {
	_, err := c.run(dir, "update-ref", "-d", turnRef(id))
	return err
}

// DiffTrees is the unified diff from one tree to another, with the flags Diff
// uses. Both sides being trees built with `add -A`, a new file arrives as an
// addition and there is no untracked pass to make.
func (c *CLI) DiffTrees(dir, from, to string) (string, error) {
	return c.capture(dir, 0, diffArgs(from, to, "--")...)
}

// Head is the commit dir has checked out. The card compares it with a merged
// pull request's head, so a reused branch name is not taken for work that
// already landed (#310).
func (c *CLI) Head(dir string) (string, error) {
	return c.run(dir, "rev-parse", "HEAD")
}
