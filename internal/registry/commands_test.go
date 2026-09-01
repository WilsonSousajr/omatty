package registry_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

func newStoreAt(t *testing.T) (*registry.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	return registry.NewStore(path), path
}

func TestAddProject_NamesTheProjectAfterTheRepoDirectory(t *testing.T) {
	store, _ := newStoreAt(t)

	// FakeGit.RepoRoot echoes the dir, so the name comes from its base.
	got, err := registry.AddProject(store, &FakeGit{}, "/p/omatty")

	if err != nil {
		t.Fatalf("AddProject() error = %v, want nil", err)
	}
	if got.Name != "omatty" || got.Root != "/p/omatty" {
		t.Errorf("project = %+v, want name omatty at /p/omatty", got)
	}
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Projects) != 1 {
		t.Errorf("state holds %d projects, want 1", len(st.Projects))
	}
}

func TestAddProject_DuplicateNamesTheProjectAndDoesNotAppend(t *testing.T) {
	store, _ := newStoreAt(t)
	if _, err := registry.AddProject(store, &FakeGit{}, "/p/omatty"); err != nil {
		t.Fatal(err)
	}

	_, err := registry.AddProject(store, &FakeGit{}, "/p/omatty")

	if err == nil {
		t.Fatal("AddProject() on a duplicate returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "omatty") {
		t.Errorf("error %q does not name the offending project", err)
	}
	st, _ := store.Load()
	if len(st.Projects) != 1 {
		t.Errorf("state holds %d projects after a duplicate, want 1", len(st.Projects))
	}
}

func TestAddSession_PersistsTheNewSession(t *testing.T) {
	store, _ := newStoreAt(t)
	git := &FakeGit{}
	if _, err := registry.AddProject(store, git, "/p/omatty"); err != nil {
		t.Fatal(err)
	}
	creator := registry.NewCreator(git, "/home/u", stubID)

	got, err := registry.AddSession(store, creator, "omatty", "parser", "parser-fix")

	if err != nil {
		t.Fatalf("AddSession() error = %v, want nil", err)
	}
	if got.ID != "fixed-uuid" || !got.Worktree {
		t.Errorf("session = %+v, want id fixed-uuid on a worktree", got)
	}
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Sessions) != 1 || st.Sessions[0].Branch != "parser-fix" {
		t.Errorf("persisted sessions = %+v, want one on parser-fix", st.Sessions)
	}
}

func TestAddSession_FailureLeavesStateUnchanged(t *testing.T) {
	store, _ := newStoreAt(t)
	creator := registry.NewCreator(&FakeGit{}, "/home/u", stubID)

	if _, err := registry.AddSession(store, creator, "ghost", "t", ""); err == nil {
		t.Fatal("AddSession() for an unknown project returned nil, want an error")
	}
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Sessions) != 0 {
		t.Errorf("state holds %d sessions after a failure, want 0", len(st.Sessions))
	}
}

// Two projects whose directories share a base name would collide, because the
// name is the key everything else looks up.
func TestAddProject_SameBaseNameInDifferentParentsIsRejected(t *testing.T) {
	store, _ := newStoreAt(t)
	if _, err := registry.AddProject(store, &FakeGit{}, "/work/api"); err != nil {
		t.Fatal(err)
	}

	_, err := registry.AddProject(store, &FakeGit{}, "/personal/api")

	if err == nil {
		t.Fatal("AddProject() for a colliding base name returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "/work/api") {
		t.Errorf("error %q does not name the already-registered root", err)
	}
}
