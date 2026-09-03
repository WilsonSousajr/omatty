package supervisor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/WilsonSousajr/omatty/internal/hooks"
	"github.com/WilsonSousajr/omatty/internal/paths"
)

// InstallHooks regenerates ~/.omatty/hooks.json for the running binary and
// returns its path. It runs before any session starts: claude refuses
// --settings on a missing file (issue #31) and the binary path moves with
// `go install`. It was four steps of logic in cmd (invariant 10, issue #79).
//
//	hooksFile, err := supervisor.InstallHooks(home, watcher.HookEventNames())
func InstallHooks(home string, eventNames []string) (string, error) {
	bin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("supervisor: locating the omatty binary: %w", err)
	}
	content, err := hooks.Render(bin, eventNames)
	if err != nil {
		return "", fmt.Errorf("supervisor: rendering hooks for %q: %w", bin, err)
	}
	path := paths.HooksFile(home)
	if err := WriteHooksFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

// WriteHooksFile writes omatty's settings file, overwriting any existing one.
//
// This reverses #31's "never overwrite": the file names the omatty binary by
// absolute path, which changes with `go install`, so it must be regenerated on
// every start. The file is ~/.omatty/hooks.json, documented as omatty's own —
// invariant 3 is about the user's ~/.claude/settings.json, which is untouched.
//
//	content, _ := hooks.Render(binPath)
//	supervisor.WriteHooksFile(paths.HooksFile(home), content)
func WriteHooksFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("supervisor: creating hooks directory %q: %w", dir, err)
	}
	if err := refuseSpecialFile(path); err != nil {
		return err
	}
	return replaceAtomically(path, content)
}

// refuseSpecialFile rejects a symlink or other non-regular file at path.
// os.WriteFile followed a planted symlink straight into the user's own
// ~/.claude/settings.json (issue #58, invariant 3). A rename would merely
// replace the link, but a file omatty expects to own must not be a link at all.
func refuseSpecialFile(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("supervisor: inspecting hooks file %q: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("supervisor: hooks file %q is a %v, want a regular file", path, fi.Mode().Type())
	}
	return nil
}

// replaceAtomically writes beside path and renames into place, so a claude
// reading --settings at the same instant never sees a truncated file (the
// #31 failure). Same pattern as registry's state.json.
func replaceAtomically(path string, content []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".hooks-*.tmp")
	if err != nil {
		return fmt.Errorf("supervisor: creating a temp file beside %q: %w", path, err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return fmt.Errorf("supervisor: writing %q: %w", f.Name(), err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("supervisor: closing %q: %w", f.Name(), err)
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("supervisor: renaming %q to %q: %w", f.Name(), path, err)
	}
	return nil
}
