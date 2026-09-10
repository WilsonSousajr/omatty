package registry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FailingWriter is a named fake whose Write always fails, standing in for a
// full or disconnected disk.
type FailingWriter struct{ Err error }

func (f *FailingWriter) Write([]byte) (int, error) { return 0, f.Err }

func TestEncodeState_WriteFailureNamesTheSessionCount(t *testing.T) {
	w := &FailingWriter{Err: errors.New("disk full")}

	err := encodeState(w, State{Sessions: []Session{{ID: "a"}, {ID: "b"}}})

	if err == nil {
		t.Fatal("encodeState() returned nil on a failing writer, want an error")
	}
	if !strings.Contains(err.Error(), "2 sessions") {
		t.Errorf("error %q does not report the session count", err)
	}
}

func TestStore_SaveWhenParentIsAFileNamesTheDirectory(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewStore(filepath.Join(blocker, "state.json"))

	err := s.Save(State{Version: Version})

	if err == nil {
		t.Fatal("Save() under a regular file returned nil, want an error")
	}
	if !strings.Contains(err.Error(), blocker) {
		t.Errorf("error %q does not name the offending directory %q", err, blocker)
	}
}

func TestStore_SaveIntoAnUnwritableDirectoryNamesThePath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.json")

	err := NewStore(path).Save(State{Version: Version})

	if err == nil {
		t.Fatal("Save() into a read-only directory returned nil, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the offending path %q", err, path)
	}
}

func TestStore_SaveOverADirectoryNamesBothPaths(t *testing.T) {
	// A directory at the state path makes the final rename fail, which is
	// the one failure that can strand a half-written registry.
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := NewStore(path).Save(State{Version: Version})

	if err == nil {
		t.Fatal("Save() over a non-empty directory returned nil, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the destination %q", err, path)
	}
}

func TestStore_LoadADirectoryNamesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := NewStore(path).Load()

	if err == nil {
		t.Fatal("Load() on a directory returned nil, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the offending file %q", err, path)
	}
}

func TestStore_SaveLeavesNoTempFileBehindAfterAFailedRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	_ = NewStore(path).Save(State{Version: Version})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".state-") {
			t.Errorf("temp file %q survived a failed save", e.Name())
		}
	}
}
