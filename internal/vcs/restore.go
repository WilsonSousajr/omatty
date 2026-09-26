package vcs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RestoreTree writes tree over dir's working tree: every file the tree holds is
// put back as it was, and every file dir has that the tree does not is removed
// (#334).
//
//	err := vcs.NewCLI().RestoreTree(sess.Dir, baseline)
//
// This is SnapshotTree read backwards, and it keeps the same discipline (#311):
// all of it happens through a *temporary* index, so the operator's own index,
// HEAD and stash are exactly where they were afterwards. A revert restores the
// working tree, and nothing else is the mechanism's to move.
//
// The removal pass asks `ls-files --others --exclude-standard` against that
// temporary index, which is what makes it safe to press: a file the turn created
// is "other" and goes, while a gitignored file that existed before the turn is
// excluded and stays. That matters more than it looks - the `.env` a session
// needs to run is exactly such a file (#309), and deleting it to undo a turn
// would break the worktree the revert was meant to rescue.
func (c *CLI) RestoreTree(dir, tree string) error {
	index, cleanup, err := c.tempIndex(dir, tree)
	if err != nil {
		return err
	}
	defer cleanup()
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := c.captureEnv(dir, 0, env, "checkout-index", "-a", "-f"); err != nil {
		return fmt.Errorf("vcs: restoring tree %s into %q: %w", tree, dir, err)
	}
	return c.removeStrays(dir, env)
}

// tempIndex builds an index holding tree alone, and the func that deletes it.
func (c *CLI) tempIndex(dir, tree string) (string, func(), error) {
	tmp, err := os.MkdirTemp("", "omatty-restore-")
	if err != nil {
		return "", nil, fmt.Errorf("vcs: making a temporary index for %q: %w", dir, err)
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	index := filepath.Join(tmp, "index")
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := c.captureEnv(dir, 0, env, "read-tree", tree); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("vcs: reading tree %s in %q: %w", tree, dir, err)
	}
	return index, cleanup, nil
}

// removeStrays deletes the files dir has that the restored index does not,
// which are the ones the turn created.
//
// -z because a file name may contain a newline, which is the whole reason the
// flag exists; splitting on "\n" would delete the wrong path on a repository
// that has one.
func (c *CLI) removeStrays(dir string, env []string) error {
	out, err := c.captureEnv(dir, 0, env, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return fmt.Errorf("vcs: listing what %q gained: %w", dir, err)
	}
	for _, rel := range strings.Split(out, "\x00") {
		if rel == "" {
			continue
		}
		if err := os.Remove(filepath.Join(dir, rel)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("vcs: removing %q from %q: %w", rel, dir, err)
		}
	}
	return nil
}
