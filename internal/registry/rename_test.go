package registry_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// seedSession registers a project and one session on it, returning the store
// and the session's id.
func seedSession(t *testing.T) (*registry.Store, string) {
	t.Helper()
	store, _ := newStoreAt(t)
	git := &FakeGit{}
	if _, err := registry.AddProject(store, git, "/p/omatty"); err != nil {
		t.Fatal(err)
	}
	sess, err := registry.AddSession(store, registry.NewCreator(git, "/home/u", stubID), "omatty", "parser", "")
	if err != nil {
		t.Fatal(err)
	}
	return store, sess.ID
}

func TestRenameSession_PersistsTheNewTitle(t *testing.T) {
	store, id := seedSession(t)

	if err := registry.RenameSession(store, id, "parser-fix"); err != nil {
		t.Fatalf("RenameSession() error = %v, want nil", err)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Sessions[0].Title != "parser-fix" {
		t.Errorf("persisted title = %q, want parser-fix", st.Sessions[0].Title)
	}
}

// The title is display-only, so a rename must not disturb the fields that
// relaunching the session depends on (invariant 9).
func TestRenameSession_TouchesNothingButTheTitle(t *testing.T) {
	store, id := seedSession(t)
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	if err := registry.RenameSession(store, id, "renamed"); err != nil {
		t.Fatal(err)
	}

	after, _ := store.Load()
	was, now := before.Sessions[0], after.Sessions[0]
	was.Title = now.Title
	if was != now {
		t.Errorf("session changed beyond its title:\n before %+v\n after  %+v", before.Sessions[0], now)
	}
}

func TestRenameSession_UnknownIDNamesItAndSavesNothing(t *testing.T) {
	store, _ := seedSession(t)

	err := registry.RenameSession(store, "ghost-uuid", "whatever")

	if err == nil {
		t.Fatal("RenameSession() for an unknown id returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "ghost-uuid") {
		t.Errorf("error %q does not name the offending id", err)
	}
	st, _ := store.Load()
	if st.Sessions[0].Title != "parser" {
		t.Errorf("title = %q after a failed rename, want the original parser", st.Sessions[0].Title)
	}
}

// An empty title would leave a blank sidebar row with no way to select it back.
func TestRenameSession_EmptyTitleIsRejected(t *testing.T) {
	store, id := seedSession(t)

	if err := registry.RenameSession(store, id, ""); err == nil {
		t.Fatal("RenameSession() with an empty title returned nil, want an error")
	}
	st, _ := store.Load()
	if st.Sessions[0].Title != "parser" {
		t.Errorf("title = %q after a rejected rename, want the original parser", st.Sessions[0].Title)
	}
}
