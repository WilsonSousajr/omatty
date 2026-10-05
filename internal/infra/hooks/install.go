package hooks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
)

// Install regenerates ~/.omatty/hooks.json for the running binary and
// returns its path. It runs before any session starts: claude refuses
// --settings on a missing file (issue #31) and the binary path moves with
// `go install`. It was four steps of logic in cmd (invariant 10, issue #79).
//
//	hooksFile, err := hooks.Install(profile, home)
//
// The events and the settings schema are the profile's (#46), and so is the
// file: paths.HooksFile names one per agent (#522).
func Install(profile agent.Profile, home string) (string, error) {
	bin, err := omattyBinary()
	if err != nil {
		return "", err
	}
	content, err := profile.RenderSettings(bin, profile.HookEvents())
	if err != nil {
		return "", fmt.Errorf("supervisor: rendering hooks for %q: %w", bin, err)
	}
	path := paths.HooksFile(home, profile.Name)
	if err := WriteSettings(path, content); err != nil {
		return "", err
	}
	return path, nil
}

// WriteSettings writes omatty's settings file, overwriting any existing one.
//
// This reverses #31's "never overwrite": the file names the omatty binary by
// absolute path, which changes with `go install`, so it must be regenerated on
// every start. The file is ~/.omatty/hooks.json, documented as omatty's own —
// invariant 3 is about the user's ~/.claude/settings.json, which is untouched.
//
//	content, _ := hooks.Render(binPath)
//	hooks.WriteSettings(paths.HooksFile(home, ""), content)
func WriteSettings(path string, content []byte) error {
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

// InstallAll installs the settings file of every agent in the catalog that
// takes hooks, and returns each one's path by agent name. An agent without
// hooks gets no file and no entry: it reports through its transcript or not
// at all (#522).
//
//	files, err := hooks.InstallAll(agents, home)
func InstallAll(agents agent.Catalog, home string) (map[string]string, error) {
	files := map[string]string{}
	for _, name := range agents.Names() {
		profile, _ := agents.Lookup(name)
		if profile.Caps.Status != agent.StatusHooks || profile.RenderSettings == nil {
			continue // no hooks, or hooks that travel as arguments (#152)
		}
		path, err := Install(profile, home)
		if err != nil {
			return nil, err
		}
		files[name] = path
	}
	return files, nil
}

// RenderAllArgs renders, for the running binary, the hook arguments of every
// agent whose hooks travel as argv rather than a settings file, by agent
// name. Codex is one: it reads no file omatty may write, only `-c` flags
// (#152). Nothing is written.
//
//	args, err := hooks.RenderAllArgs(agents)
func RenderAllArgs(agents agent.Catalog) (map[string][]string, error) {
	bin, err := omattyBinary()
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, name := range agents.Names() {
		profile, _ := agents.Lookup(name)
		if profile.Caps.Status != agent.StatusHooks || profile.RenderArgs == nil {
			continue
		}
		args, err := profile.RenderArgs(bin, profile.HookEvents())
		if err != nil {
			return nil, fmt.Errorf("supervisor: rendering %s's hook arguments for %q: %w", name, bin, err)
		}
		out[name] = args
	}
	return out, nil
}

// omattyBinary is the running omatty, which every hook route names by
// absolute path: an agent runs its hooks with whatever PATH it inherited.
func omattyBinary() (string, error) {
	bin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("supervisor: locating the omatty binary: %w", err)
	}
	return bin, nil
}
