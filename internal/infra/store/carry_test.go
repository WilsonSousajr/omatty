package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/store"
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

	if err := store.CarryInto(dst, src, []string{".env", "run.sh", "certs"}); err != nil {
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

	if err := store.CarryInto(dst, src, []string{"certs"}); err != nil {
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

	if err := store.CarryInto(dst, src, []string{".env"}); err != nil {
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
		if err := store.CarryInto(dst, src, []string{bad}); err == nil {
			t.Errorf("CarryInto accepted %q", bad)
		}
	}
}

// A listed path that is simply not there is skipped, not fatal: a list outlives
// the files it names, and refusing to create the session would be worse (#309).
func TestCarryInto_skipsAMissingSource_issue309(t *testing.T) {
	src, dst := carryRepo(t), t.TempDir()

	if err := store.CarryInto(dst, src, []string{"gone.env", ".env"}); err != nil {
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
