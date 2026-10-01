package sessions_test

import (
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/store"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// carryRepo is a main checkout holding two gitignored files a worktree needs,
// one of them nested. The copy itself is internal/infra/store's, and its
// tests cover modes and links; here it only has to have happened.
func carryRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range map[string]string{".env": "TOKEN=shh", "certs/ca/root.pem": "PEM"} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The copy runs as part of creating the worktree, before the session is
// registered and so before claude can start in it. ccmanager orders it the
// same way and for the same reason: whatever runs next must be able to rely
// on the files (#309).
func TestCreator_carriesTheProjectsFilesIntoANewWorktree_issue309(t *testing.T) {
	root, wtRoot := carryRepo(t), t.TempDir()
	st := &session.State{
		Version:  session.Version,
		Projects: []session.Project{{Name: "omatty", Root: root, Carry: []string{".env", "certs"}}},
	}

	sess, err := sessions.NewCreator(&FakeGit{}, sessions.CreatorOpts{WorktreeDir: paths.WorktreeDir, WorktreeRoot: wtRoot, Carry: store.CarryInto}, stubID).
		CreateWorktree(t.Context(), st, "omatty", "poke", "topic")
	if err != nil {
		t.Fatal(err)
	}

	if got := read(t, filepath.Join(sess.Dir, ".env")); got != "TOKEN=shh" {
		t.Errorf("the worktree's .env = %q", got)
	}
	if got := read(t, filepath.Join(sess.Dir, "certs", "ca", "root.pem")); got != "PEM" {
		t.Errorf("the worktree's nested cert = %q", got)
	}
	if len(st.Sessions) != 1 {
		t.Errorf("state holds %d sessions, want 1", len(st.Sessions))
	}
}

// A carry that fails takes the worktree with it. Leaving a half-populated
// worktree registered would hand claude a directory the operator did not
// choose, and Create's promise is that a failure leaves st untouched (#309).
func TestCreator_rollsBackTheWorktreeWhenACarryFails_issue309(t *testing.T) {
	g := &FakeGit{}
	st := &session.State{
		Version:  session.Version,
		Projects: []session.Project{{Name: "omatty", Root: carryRepo(t), Carry: []string{"../escape"}}},
	}

	_, err := sessions.NewCreator(g, sessions.CreatorOpts{WorktreeDir: paths.WorktreeDir, WorktreeRoot: t.TempDir(), Carry: store.CarryInto}, stubID).
		CreateWorktree(t.Context(), st, "omatty", "poke", "topic")

	if err == nil {
		t.Fatal("CreateWorktree() error = nil, want the refused carry path")
	}
	if len(g.Removed) != 1 {
		t.Errorf("the worktree was removed %d times, want 1 - a failed carry must not leave one behind", len(g.Removed))
	}
	if len(st.Sessions) != 0 {
		t.Errorf("state holds %d sessions, want 0 after a failure", len(st.Sessions))
	}
}

// The list has to survive a write and a reload, because every worktree after
// this one reads it back from the file (#309).
func TestSetCarry_roundTripsThroughTheStateFile_issue309(t *testing.T) {
	store, _ := storeWithProject(t, "omatty")

	if err := sessions.SetCarry(t.Context(), store, "omatty", []string{".env", "certs"}); err != nil {
		t.Fatalf("SetCarry() error = %v", err)
	}

	st, err := store.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Projects[0].Carry; len(got) != 2 || got[0] != ".env" || got[1] != "certs" {
		t.Errorf("Carry = %v, want [.env certs]", got)
	}
}

// Nil, never an empty array: a project with nothing to carry omits the key, so
// a file written before #309 needs no migration and Version stays 1.
func TestClearCarry_omitsTheKeyRatherThanWritingAnEmptyList_issue309(t *testing.T) {
	store, path := storeWithProject(t, "omatty")
	if err := sessions.SetCarry(t.Context(), store, "omatty", []string{".env"}); err != nil {
		t.Fatal(err)
	}

	if err := sessions.ClearCarry(t.Context(), store, "omatty"); err != nil {
		t.Fatalf("ClearCarry() error = %v", err)
	}

	raw := read(t, path)
	if strings.Contains(raw, "carry") {
		t.Errorf("state.json still carries the key:\n%s", raw)
	}
	st, err := store.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if st.Projects[0].Carry != nil {
		t.Errorf("Carry = %v, want nil", st.Projects[0].Carry)
	}
}

// Setting a list on a project that is not registered names it rather than
// writing a row nothing points at.
func TestSetCarry_refusesAnUnknownProject_issue309(t *testing.T) {
	store, _ := storeWithProject(t, "omatty")

	err := sessions.SetCarry(t.Context(), store, "nope", []string{".env"})

	if err == nil {
		t.Fatal("SetCarry() error = nil, want an unknown-project error")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the project", err)
	}
}

// The copy is injected since migration step 5.4 (#653). A project that lists
// files to carry must not silently get none because nothing was wired: the
// create fails naming the project, and the worktree goes with it.
func TestCreator_refusesACarryWithNoCopierWired_issue653(t *testing.T) {
	g := &FakeGit{}
	st := &session.State{
		Version:  session.Version,
		Projects: []session.Project{{Name: "omatty", Root: carryRepo(t), Carry: []string{".env"}}},
	}

	_, err := sessions.NewCreator(g, sessions.CreatorOpts{WorktreeDir: paths.WorktreeDir, WorktreeRoot: t.TempDir()}, stubID).
		CreateWorktree(t.Context(), st, "omatty", "poke", "topic")

	if err == nil || !strings.Contains(err.Error(), "omatty") {
		t.Fatalf("CreateWorktree() error = %v, want one naming the project", err)
	}
	if len(g.Removed) != 1 || len(st.Sessions) != 0 {
		t.Errorf("removed %d worktrees and registered %d sessions, want 1 and 0", len(g.Removed), len(st.Sessions))
	}
}
