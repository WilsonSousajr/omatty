package crap_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/crap"
	"github.com/WilsonSousajr/omatty/internal/golist"
)

// A test added after the profile changes coverage without touching one line of
// source, so the mtime comparison the guard makes against GoFiles alone stays
// quiet and the gate scores the old profile. It then reports a covered
// function at 0% - a confident wrong number that reads exactly like a real
// finding, which is what this guard exists to prevent (#385).
func TestNewerSource_findsATestFileEditedAfterTheProfile_issue385(t *testing.T) {
	dir := t.TempDir()
	// The source is older than the profile and the test is newer: exactly the
	// sequence that fooled the guard - run the gate, then add a test.
	write(t, filepath.Join(dir, "a.go"), "package a\n")
	write(t, filepath.Join(dir, "a_test.go"), "package a\n")
	cutoff := time.Now().Add(-time.Hour)
	backdate(t, filepath.Join(dir, "a.go"), cutoff.Add(-time.Hour))

	name, found, err := crap.NewerSource([]golist.Package{{
		ImportPath: "m/a", Dir: dir, GoFiles: []string{"a.go"}, TestGoFiles: []string{"a_test.go"},
	}}, cutoff)

	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("NewerSource() found nothing, but a_test.go was written after the cutoff")
	}
	if filepath.Base(name) != "a_test.go" {
		t.Errorf("NewerSource() = %q, want it to name a_test.go", name)
	}
}

// The external test package is the case that actually bit: #309's tests were
// `package registry_test`, which go list reports as XTestGoFiles, so neither
// GoFiles nor TestGoFiles would have caught them (#385).
func TestNewerSource_findsAnExternalTestFileEditedAfterTheProfile_issue385(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.go"), "package a\n")
	write(t, filepath.Join(dir, "x_test.go"), "package a_test\n")
	cutoff := time.Now().Add(-time.Hour)
	backdate(t, filepath.Join(dir, "a.go"), cutoff.Add(-time.Hour))

	name, found, err := crap.NewerSource([]golist.Package{{
		ImportPath: "m/a", Dir: dir, GoFiles: []string{"a.go"}, XTestGoFiles: []string{"x_test.go"},
	}}, cutoff)

	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("NewerSource() found nothing, but x_test.go was written after the cutoff")
	}
	if filepath.Base(name) != "x_test.go" {
		t.Errorf("NewerSource() = %q, want it to name x_test.go", name)
	}
}

// backdate makes a file older than the cutoff, so a test asserting which file
// the guard names is not answered by whichever one happened to be written
// first.
func backdate(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}
