package registry_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// adoptStore is a store holding one project, which is what adoption registers
// a session against.
func adoptStore(t *testing.T) *registry.Store {
	t.Helper()
	s := registry.NewStore(filepath.Join(t.TempDir(), "state.json"))
	if _, err := registry.AddProject(s, &FakeGit{}, "/p/omatty"); err != nil {
		t.Fatal(err)
	}
	return s
}

// The session already exists in claude's world; adoption is omatty learning
// about it. Worktree is false and that is load-bearing rather than incidental:
// omatty did not create that directory, so archive must never offer to delete
// it - the rule archiveChoices already applies to a main-checkout session (#40).
func TestAdoptSession_RegistersASessionOmattyDoesNotOwnTheDirectoryOf_issue122(t *testing.T) {
	s := adoptStore(t)

	sess, err := registry.AdoptSession(s, "abc-123", "omatty", "fix the parser", "/p/omatty")

	if err != nil {
		t.Fatalf("AdoptSession() error = %v, want nil", err)
	}
	if sess.Worktree {
		t.Error("Worktree = true; omatty did not create this directory and must never delete it")
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Sessions) != 1 {
		t.Fatalf("state holds %d sessions, want 1", len(st.Sessions))
	}
	got := st.Sessions[0]
	if got.ID != "abc-123" || got.Project != "omatty" || got.Dir != "/p/omatty" {
		t.Errorf("persisted %+v, want the adopted id, project and directory", got)
	}
}

// Invariant 9: state.json must suffice to relaunch every session. An adopted
// row carries the same fields as any other, so `claude --resume <id>` in Dir is
// enough - which is what the launcher already does once a transcript exists.
func TestAdoptSession_PersistsEnoughToRelaunch_invariant9(t *testing.T) {
	s := adoptStore(t)

	if _, err := registry.AdoptSession(s, "abc-123", "omatty", "one", "/p/omatty"); err != nil {
		t.Fatal(err)
	}

	st, _ := s.Load()
	sess := st.Sessions[0]
	if sess.ID == "" || sess.Dir == "" {
		t.Errorf("session %+v cannot be relaunched: --resume needs an id and a directory", sess)
	}
}

// Adopting a session omatty already holds would give two sidebar rows one
// process, and the second would fight the first for the PTY.
func TestAdoptSession_RefusesADuplicateID_issue122(t *testing.T) {
	s := adoptStore(t)
	if _, err := registry.AdoptSession(s, "abc-123", "omatty", "one", "/p/omatty"); err != nil {
		t.Fatal(err)
	}

	_, err := registry.AdoptSession(s, "abc-123", "omatty", "two", "/p/omatty")

	if err == nil {
		t.Fatal("AdoptSession() with an id already registered returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "abc-123") {
		t.Errorf("error %q does not name the offending id", err)
	}
	st, _ := s.Load()
	if len(st.Sessions) != 1 {
		t.Errorf("state holds %d sessions, want the failed adoption to have changed nothing", len(st.Sessions))
	}
}

// The same guard as rename: a title of nothing but spaces renders as empty a
// sidebar row as "" does, and passed an == "" check (#41).
func TestAdoptSession_RefusesABlankTitle_issue122(t *testing.T) {
	s := adoptStore(t)

	_, err := registry.AdoptSession(s, "abc-123", "omatty", "   ", "/p/omatty")

	if err == nil {
		t.Fatal("AdoptSession() with a whitespace title returned nil, want an error")
	}
}

// A session must belong to a project the registry knows, or the sidebar would
// hold a row under a heading that does not exist.
func TestAdoptSession_RefusesAnUnknownProject_issue122(t *testing.T) {
	s := adoptStore(t)

	_, err := registry.AdoptSession(s, "abc-123", "nope", "one", "/p/nope")

	if err == nil {
		t.Fatal("AdoptSession() with an unregistered project returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the unknown project", err)
	}
}
