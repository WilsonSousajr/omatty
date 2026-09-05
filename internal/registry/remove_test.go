package registry_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// seedTwoSessions registers one project with two sessions, the second on a
// worktree, and returns the store and both ids in creation order.
func seedTwoSessions(t *testing.T) (*registry.Store, string, string) {
	t.Helper()
	store, _ := newStoreAt(t)
	git := &FakeGit{}
	if _, err := registry.AddProject(store, git, "/p/omatty"); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, 2)
	for i, branch := range []string{"", "parser-fix"} {
		n := i
		c := registry.NewCreator(git, "/home/u", func() string { return string(rune('a' + n)) })
		sess, err := registry.AddSession(store, c, "omatty", "session-"+string(rune('a'+n)), branch)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, sess.ID)
	}
	return store, ids[0], ids[1]
}

func TestRemoveSession_DropsTheRowAndLeavesTheOthers(t *testing.T) {
	store, first, second := seedTwoSessions(t)

	got, err := registry.RemoveSession(store, first)

	if err != nil {
		t.Fatalf("RemoveSession() error = %v, want nil", err)
	}
	if got.ID != first {
		t.Errorf("returned session id = %q, want %q", got.ID, first)
	}
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Sessions) != 1 || st.Sessions[0].ID != second {
		t.Errorf("sessions after removal = %+v, want only %s", st.Sessions, second)
	}
}

// The caller needs the removed session's own fields to decide whether a
// worktree may be deleted, so they must survive the call (#40).
func TestRemoveSession_ReturnsTheRemovedSessionsWorktreeFields(t *testing.T) {
	store, _, second := seedTwoSessions(t)

	got, err := registry.RemoveSession(store, second)

	if err != nil {
		t.Fatal(err)
	}
	if !got.Worktree || got.Dir == "" || got.Branch != "parser-fix" {
		t.Errorf("returned session = %+v, want a worktree on parser-fix with a dir", got)
	}
}

func TestRemoveSession_UnknownIDNamesItAndChangesNothing(t *testing.T) {
	store, _, _ := seedTwoSessions(t)

	_, err := registry.RemoveSession(store, "ghost-uuid")

	if err == nil {
		t.Fatal("RemoveSession() for an unknown id returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "ghost-uuid") {
		t.Errorf("error %q does not name the offending id", err)
	}
	st, _ := store.Load()
	if len(st.Sessions) != 2 {
		t.Errorf("state holds %d sessions after a failed removal, want 2", len(st.Sessions))
	}
}

// Removing the last session must leave a valid, empty registry rather than a
// state.json that no longer parses.
func TestRemoveSession_RemovingEveryoneLeavesAnEmptyRegistry(t *testing.T) {
	store, first, second := seedTwoSessions(t)

	for _, id := range []string{first, second} {
		if _, err := registry.RemoveSession(store, id); err != nil {
			t.Fatalf("RemoveSession(%s) error = %v, want nil", id, err)
		}
	}

	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after removing every session: %v", err)
	}
	if len(st.Sessions) != 0 || len(st.Projects) != 1 {
		t.Errorf("state = %+v, want no sessions and the project still registered", st)
	}
}
