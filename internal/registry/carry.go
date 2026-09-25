// Carrying a project's gitignored files into a new worktree (#309).
//
// A worktree is a clean checkout, so everything git does not track is absent
// from it: .env, local certificates, generated config. M9 made the gate the
// product, and a gate step that fails because .env is missing is the gate
// being wrong about the code - a red card the operator learns to ignore,
// which is worse than no card. So the copy runs before the session starts,
// not as a hook the operator remembers to write.
//
// The list lives in state.json as Project.Carry, beside Project.Gate, rather
// than in a repository file the way ccmanager's .worktreeinclude and fleet's
// .fleet.json do. Three reasons: it is how every other per-project setting
// here already works, so there is one place to look; the empty value is
// derivable, so no migration and Version stays 1 (invariant 9); and a cloned
// repository must not get to decide which files are copied off this
// operator's disk. The cost is real and worth naming - the list is not shared
// with a team, and each person sets their own.

package registry

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// SetCarry records the gitignored paths copied into each new worktree of a
// project, replacing whatever it had.
//
//	err := registry.SetCarry(store, "omatty", []string{".env", "certs"})
func SetCarry(s *Store, project string, paths []string) error {
	return editCarry(s, project, paths)
}

// ClearCarry forgets a project's carry list, returning it to "nothing to
// carry" - which is the nil Project.Carry already means.
func ClearCarry(s *Store, project string) error {
	return editCarry(s, project, nil)
}

// editCarry is the load-find-write both commands share, as editGate is for
// the gate.
func editCarry(s *Store, project string, paths []string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	if _, err := findProject(&st, project); err != nil {
		return err
	}
	for i := range st.Projects {
		if st.Projects[i].Name == project {
			st.Projects[i].Carry = paths
		}
	}
	return s.Save(st)
}

// CarryInto copies each of root's listed paths into dir, which is a freshly
// created worktree.
//
// A missing entry is skipped rather than fatal: a list outlives the files it
// names, and refusing to create the session over a stale entry would be worse
// than starting without it. A destination that already exists is left alone,
// because git put it there and the main checkout's copy is not more correct.
//
//	err := registry.CarryInto(sess.Dir, p.Root, p.Carry)
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
