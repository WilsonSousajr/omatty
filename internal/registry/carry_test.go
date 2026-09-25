package registry_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// carryRepo is a main checkout holding the gitignored files a worktree needs:
// a plain file, an executable, a directory with a nested file, and a symlink.
func carryRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "TOKEN=shh", 0o600)
	write(t, filepath.Join(root, "run.sh"), "#!/bin/sh\n", 0o755)
	if err := os.MkdirAll(filepath.Join(root, "certs", "ca"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "certs", "ca", "root.pem"), "PEM", 0o644)
	if err := os.Symlink("../.env", filepath.Join(root, "certs", "link")); err != nil {
		t.Fatal(err)
	}
	return root
}

func write(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// A worktree is a checkout without the operator's gitignored files, so a gate
// step fails for a reason that has nothing to do with the code (#309).
func TestCarryInto_copiesFilesDirectoriesAndModes_issue309(t *testing.T) {
	src, dst := carryRepo(t), t.TempDir()

	if err := registry.CarryInto(dst, src, []string{".env", "run.sh", "certs"}); err != nil {
		t.Fatal(err)
	}

	if got := read(t, filepath.Join(dst, ".env")); got != "TOKEN=shh" {
		t.Errorf(".env = %q", got)
	}
	if got := read(t, filepath.Join(dst, "certs", "ca", "root.pem")); got != "PEM" {
		t.Errorf("nested file = %q", got)
	}
	info, err := os.Stat(filepath.Join(dst, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("run.sh mode = %v, want 0755 - an executable that lost its bit will not run", info.Mode().Perm())
	}
}

// A symlink is copied as a link. Following it would turn a relative link into
// a second copy of the file it points at, and an absolute one into a path that
// escapes the worktree entirely (#309).
func TestCarryInto_copiesASymlinkAsALink_issue309(t *testing.T) {
	src, dst := carryRepo(t), t.TempDir()

	if err := registry.CarryInto(dst, src, []string{"certs"}); err != nil {
		t.Fatal(err)
	}

	target, err := os.Readlink(filepath.Join(dst, "certs", "link"))
	if err != nil {
		t.Fatalf("the symlink was not carried as a link: %v", err)
	}
	if target != "../.env" {
		t.Errorf("link target = %q, want %q", target, "../.env")
	}
}

// A tracked file wins: the worktree's own copy is what git checked out, and
// overwriting it with the main checkout's would be a silent edit (#309).
func TestCarryInto_doesNotOverwriteWhatIsAlreadyThere_issue309(t *testing.T) {
	src, dst := carryRepo(t), t.TempDir()
	write(t, filepath.Join(dst, ".env"), "TRACKED", 0o644)

	if err := registry.CarryInto(dst, src, []string{".env"}); err != nil {
		t.Fatal(err)
	}

	if got := read(t, filepath.Join(dst, ".env")); got != "TRACKED" {
		t.Errorf(".env = %q, want the destination's own copy kept", got)
	}
}

// A path that is absolute or climbs out of the checkout is refused, the same
// guard review.ReadPreview applies for the same reason (#309).
func TestCarryInto_refusesAPathLeavingTheCheckout_issue309(t *testing.T) {
	src, dst := carryRepo(t), t.TempDir()

	for _, bad := range []string{"/etc/passwd", "../outside", "certs/../../escape", ".."} {
		if err := registry.CarryInto(dst, src, []string{bad}); err == nil {
			t.Errorf("CarryInto accepted %q", bad)
		}
	}
}

// A listed path that is simply not there is skipped, not fatal: a list outlives
// the files it names, and refusing to create the session would be worse (#309).
func TestCarryInto_skipsAMissingSource_issue309(t *testing.T) {
	src, dst := carryRepo(t), t.TempDir()

	if err := registry.CarryInto(dst, src, []string{"gone.env", ".env"}); err != nil {
		t.Fatalf("a missing entry should be skipped, got %v", err)
	}
	if got := read(t, filepath.Join(dst, ".env")); got != "TOKEN=shh" {
		t.Errorf("the entry after the missing one was not carried: %q", got)
	}
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
	st := &registry.State{
		Version:  registry.Version,
		Projects: []registry.Project{{Name: "omatty", Root: root, Carry: []string{".env", "certs"}}},
	}

	sess, err := registry.NewCreator(&FakeGit{}, registry.CreatorOpts{WorktreeRoot: wtRoot}, stubID).
		CreateWorktree(st, "omatty", "poke", "topic")
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
	st := &registry.State{
		Version:  registry.Version,
		Projects: []registry.Project{{Name: "omatty", Root: carryRepo(t), Carry: []string{"../escape"}}},
	}

	_, err := registry.NewCreator(g, registry.CreatorOpts{WorktreeRoot: t.TempDir()}, stubID).
		CreateWorktree(st, "omatty", "poke", "topic")

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

	if err := registry.SetCarry(store, "omatty", []string{".env", "certs"}); err != nil {
		t.Fatalf("SetCarry() error = %v", err)
	}

	st, err := store.Load()
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
	if err := registry.SetCarry(store, "omatty", []string{".env"}); err != nil {
		t.Fatal(err)
	}

	if err := registry.ClearCarry(store, "omatty"); err != nil {
		t.Fatalf("ClearCarry() error = %v", err)
	}

	raw := read(t, path)
	if strings.Contains(raw, "carry") {
		t.Errorf("state.json still carries the key:\n%s", raw)
	}
	st, err := store.Load()
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

	err := registry.SetCarry(store, "nope", []string{".env"})

	if err == nil {
		t.Fatal("SetCarry() error = nil, want an unknown-project error")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the project", err)
	}
}
