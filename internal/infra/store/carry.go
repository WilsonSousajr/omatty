// Copying a project's gitignored files into a new worktree (#309): the file
// half of carry, moved here from internal/registry because copying files is
// infra's business (ADR 0001, migration step 5.4, #653). Which files, and
// when, is the session service's; internal/service/sessions/carry.go says why the
// list lives in state.json.

package store

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// CarryInto copies each of root's listed paths into dir, which is a freshly
// created worktree.
//
// A missing entry is skipped rather than fatal: a list outlives the files it
// names, and refusing to create the session over a stale entry would be worse
// than starting without it. A destination that already exists is left alone,
// because git put it there and the main checkout's copy is not more correct.
//
//	err := store.CarryInto(sess.Dir, p.Root, p.Carry)
func CarryInto(dir, root string, paths []string) error {
	for _, rel := range paths {
		clean, err := carryPath(rel)
		if err != nil {
			return err
		}
		src := filepath.Join(root, clean)
		if _, err := os.Lstat(src); err != nil {
			slog.Warn("carry skipped a path the checkout does not have",
				"project_root", root, "path", clean, "err", err)
			continue
		}
		if err := copyTree(src, filepath.Join(dir, clean)); err != nil {
			return fmt.Errorf("registry: carrying %q into worktree %q: %w", clean, dir, err)
		}
	}
	return nil
}

// carryPath refuses anything absolute or climbing out of the checkout, the
// guard review.ReadPreview applies for the same reason: a carry list must
// never reach outside the repository it belongs to.
func carryPath(rel string) (string, error) {
	clean := filepath.Clean(rel)
	if clean == "" || clean == "." {
		return "", fmt.Errorf("registry: carry path %q names nothing", rel)
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("registry: carry path %q leaves the project's checkout", rel)
	}
	return clean, nil
}

// copyTree copies one entry - file, directory or symlink - preserving mode and
// never following a link. A link is recreated as a link: following a relative
// one would make a second copy of its target, and an absolute one would point
// out of the worktree.
func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return copyLink(src, dst)
	case info.IsDir():
		return copyDir(src, dst, info)
	default:
		return copyFile(src, dst, info)
	}
}

// copyLink recreates a symlink, leaving one that is already there alone.
func copyLink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(dst); err == nil {
		return nil
	}
	return os.Symlink(target, dst)
}

// copyDir walks a directory's entries. The directory itself is created if
// absent, and an existing one is reused rather than refused: only files are
// left alone, and a shared parent is not a conflict.
func copyDir(src, dst string, info os.FileInfo) error {
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies one file's bytes and mode, leaving a destination that
// already exists alone - git checked that one out.
func copyFile(src, dst string, info os.FileInfo) error {
	if _, err := os.Lstat(dst); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return writeCopy(src, dst, info.Mode().Perm())
}

// writeCopy streams src's bytes into a new dst with mode. O_EXCL rather than
// a truncate: copyFile has already decided dst is absent, and a race that
// created it in between must lose to whatever put it there.
func writeCopy(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
