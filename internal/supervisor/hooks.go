package supervisor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// defaultHooks is omatty's settings file before M2 fills in the real hooks.
// It is deliberately minimal: invariant 3 says omatty ships its own settings
// rather than touching the user's, and --settings is additive, so an empty
// hooks block changes nothing about how claude behaves.
const defaultHooks = "{\n  \"hooks\": {}\n}\n"

// EnsureHooksFile writes omatty's settings file if it is absent.
//
// claude refuses to start when --settings names a missing file ("Error:
// Settings file not found"), which left every session with a dead PTY behind
// its sidebar row (issue #31). An existing file is never overwritten: it is
// omatty's, but an operator may still have edited it.
//
//	if err := supervisor.EnsureHooksFile(paths.HooksFile(home)); err != nil { ... }
func EnsureHooksFile(path string) error {
	switch _, err := os.Stat(path); {
	case err == nil:
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("supervisor: cannot read hooks file %q: %w", path, err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("supervisor: creating hooks directory %q: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte(defaultHooks), 0o600); err != nil {
		return fmt.Errorf("supervisor: writing hooks file %q: %w", path, err)
	}
	return nil
}
