package supervisor

import (
	"fmt"
	"os"
	"path/filepath"
)

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
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("supervisor: writing hooks file %q: %w", path, err)
	}
	return nil
}
