package registry_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// seedThreeProjects registers a, b and c in that order, with one session in b.
func seedThreeProjects(t *testing.T) *registry.Store {
	t.Helper()
	store, _ := newStoreAt(t)
	git := &FakeGit{}
	for _, dir := range []string{"/p/a", "/p/b", "/p/c"} {
		if _, err := registry.AddProject(store, git, dir); err != nil {
			t.Fatal(err)
		}
	}
	c := registry.NewCreator(git, registry.CreatorOpts{WorktreeRoot: "/wt"}, func() string { return "b-1" })
	if _, err := registry.AddSession(store, c, "b", "t", ""); err != nil {
		t.Fatal(err)
	}
	return store
}

func registeredNames(t *testing.T, store *registry.Store) string {
	t.Helper()
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(st.Projects))
	for _, p := range st.Projects {
		names = append(names, p.Name)
	}
	return strings.Join(names, ",")
}

func TestRemoveProject_DropsAnEmptyProjectAndKeepsTheOrder_issue159(t *testing.T) {
	store := seedThreeProjects(t)

	got, err := registry.RemoveProject(store, "a")

	if err != nil {
		t.Fatalf("RemoveProject(a) error = %v, want nil", err)
	}
	if got.Name != "a" || got.Root != "/p/a" {
		t.Errorf("returned project = %+v, want a at /p/a", got)
	}
	if names := registeredNames(t, store); names != "b,c" {
		t.Errorf("projects after removal = %q, want b,c in registration order", names)
	}
}

func TestRemoveProject_RefusesWhileSessionsExistAndChangesNothing_issue159(t *testing.T) {
	store := seedThreeProjects(t)

	_, err := registry.RemoveProject(store, "b")

	if err == nil {
		t.Fatal("RemoveProject(b) succeeded with a session in it, want an error")
	}
	if !strings.Contains(err.Error(), `"b"`) || !strings.Contains(err.Error(), "1 session") {
		t.Errorf("error %q does not name the project and the session count", err)
	}
	if names := registeredNames(t, store); names != "a,b,c" {
		t.Errorf("projects after a refused removal = %q, want all three", names)
	}
}

func TestRemoveProject_UnknownNameIsAnError_issue159(t *testing.T) {
	store := seedThreeProjects(t)

	_, err := registry.RemoveProject(store, "ghost")

	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Errorf("RemoveProject(ghost) error = %v, want one naming ghost", err)
	}
}

// omatty did not create the repository and does not own it: removal forgets
// the row and touches nothing on disk (#159, the line #122 drew for adoption).
func TestRemoveProject_LeavesTheRepositoryUntouched_issue159(t *testing.T) {
	store, _ := newStoreAt(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "README.md")
	if err := os.WriteFile(marker, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.AddProject(store, &FakeGit{}, dir); err != nil {
		t.Fatal(err)
	}

	if _, err := registry.RemoveProject(store, filepath.Base(dir)); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("the repository was touched: %v", err)
	}
}

func TestRemoveProject_ARemovedProjectCanBeRegisteredAgain_issue159(t *testing.T) {
	store := seedThreeProjects(t)
	if _, err := registry.RemoveProject(store, "c"); err != nil {
		t.Fatal(err)
	}

	p, err := registry.AddProject(store, &FakeGit{}, "/p/c")

	if err != nil || p.Name != "c" {
		t.Errorf("re-registering c after removal = %+v, %v; want c registered again", p, err)
	}
}
