package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Store loads and saves State at a fixed path.
//
//	s := registry.NewStore(paths.StateFile(home))
//	state, err := s.Load()
type Store struct{ path string }

// NewStore returns a Store backed by the file at path.
func NewStore(path string) *Store { return &Store{path: path} }

// Load reads the state file. A missing file is not an error: it yields an
// empty state, which is what a first run should see.
func (s *Store) Load() (State, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return State{Version: Version}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("registry: reading state file %q: %w", s.path, err)
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, fmt.Errorf(
			"registry: state file %q is not a JSON State object (want {version,projects,sessions}): %w",
			s.path, err)
	}
	return st, nil
}

// Save writes the state atomically, so a crash mid-write cannot leave a
// truncated registry that would strand running sessions (invariant 9).
func (s *Store) Save(st State) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("registry: creating state directory %q: %w", dir, err)
	}
	return atomicWrite(s.path, st)
}

// atomicWrite replaces path in one rename, so a reader never sees a partial
// registry.
func atomicWrite(path string, st State) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("registry: creating a temp file beside %q: %w", path, err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if err := encodeAndSync(f, st); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("registry: renaming %q to %q: %w", f.Name(), path, err)
	}
	return nil
}

// encodeAndSync writes st and flushes it to disk before the caller renames
// it into place. Without the Sync a crash can leave an empty file holding
// the registry's name (invariant 9).
func encodeAndSync(f *os.File, st State) error {
	defer func() { _ = f.Close() }()
	if err := encodeState(f, st); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("registry: syncing %q: %w", f.Name(), err)
	}
	return nil
}

// encodeState writes st as indented JSON. It takes an io.Writer rather than
// the file so a failing write is reachable from a test.
func encodeState(w io.Writer, st State) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(st); err != nil {
		return fmt.Errorf("registry: writing state with %d sessions: %w", len(st.Sessions), err)
	}
	return nil
}
