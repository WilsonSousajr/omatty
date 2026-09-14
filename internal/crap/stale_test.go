package crap_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/crap"
	"github.com/WilsonSousajr/omatty/internal/golist"
)

// Reusing the coverage step's profile is what makes this gate nearly free, and
// it is also the one way it can report a confident wrong number: an old profile
// against an edited tree attributes blocks to whatever now sits at those lines.
// Refusing beats reporting.
func TestNewerSource_findsASourceFileEditedAfterTheProfile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.go"), "package a\n")
	cutoff := time.Now().Add(-time.Hour)

	name, found, err := crap.NewerSource(pkgIn(dir, "a.go"), cutoff)

	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("NewerSource() found nothing, but a.go was written after the cutoff")
	}
	if filepath.Base(name) != "a.go" {
		t.Errorf("NewerSource() = %q, want it to name a.go", name)
	}
}

// And it must not cry wolf, or the gate becomes a step people pass -force to.
func TestNewerSource_saysNothingWhenTheProfileIsCurrent(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.go"), "package a\n")
	cutoff := time.Now().Add(time.Hour)

	_, found, err := crap.NewerSource(pkgIn(dir, "a.go"), cutoff)

	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("NewerSource() reported a stale profile against an untouched tree")
	}
}

// A file go list named but that is not there is an error, not a quiet pass.
func TestNewerSource_missingFileIsAnError(t *testing.T) {
	_, _, err := crap.NewerSource(pkgIn(t.TempDir(), "gone.go"), time.Now())

	if err == nil {
		t.Fatal("NewerSource() error = nil, want the missing file surfaced")
	}
}

func pkgIn(dir string, files ...string) []golist.Package {
	return []golist.Package{{ImportPath: "m/a", Dir: dir, GoFiles: files}}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
