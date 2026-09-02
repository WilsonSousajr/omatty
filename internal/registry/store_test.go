package registry_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

func TestStore_LoadMissingFileReturnsEmptyState(t *testing.T) {
	s := registry.NewStore(filepath.Join(t.TempDir(), "state.json"))
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load() on a missing file returned error %v, want nil", err)
	}
	if len(got.Sessions) != 0 || len(got.Projects) != 0 {
		t.Errorf("Load() = %+v, want an empty state", got)
	}
}

func TestStore_SaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	s := registry.NewStore(path)
	want := registry.State{
		Version:  1,
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}},
		Sessions: []registry.Session{{
			ID: "abc-123", Project: "omatty", Title: "parser fix",
			Dir: "/w/parser-fix", Branch: "parser-fix", Worktree: true,
		}},
	}
	if err := s.Save(want); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(got.Sessions) != 1 || got.Sessions[0].ID != "abc-123" {
		t.Errorf("Load() = %+v, want the saved session", got)
	}
	if got.Sessions[0].Dir != "/w/parser-fix" {
		t.Errorf("Dir = %q, want %q", got.Sessions[0].Dir, "/w/parser-fix")
	}
}

func TestStore_LoadMalformedJSONNamesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := registry.NewStore(path).Load()
	if err == nil {
		t.Fatal("Load() on malformed JSON returned nil error, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the offending file %q", err, path)
	}
}

func TestStore_SaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	s := registry.NewStore(filepath.Join(dir, "state.json"))
	if err := s.Save(registry.State{Version: 1}); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Errorf("directory holds %v, want only state.json", entries)
	}
}

// Invariant 2: a session's live status is derived, never persisted, so a
// stale "thinking" cannot survive a restart.
func TestSession_HasNoPersistedStatusField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := registry.NewStore(path).Save(registry.State{
		Version:  1,
		Sessions: []registry.Session{{ID: "abc-123"}},
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "status") {
		t.Errorf("state.json contains a status field:\n%s", b)
	}
}
