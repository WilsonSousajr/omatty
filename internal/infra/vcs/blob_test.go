package vcs_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/vcs"
)

// The old side of a diff is a file as it was at the diff's base, read whole to
// prime the highlighter (#800): at a commit, and at a turn baseline's tree.
func TestCLI_BlobReadsAFileAtARevision_issue800(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "p.go", "package p\n\nvar s = `\n")
	gitOut(t, dir, "add", "p.go")
	gitOut(t, dir, "commit", "-m", "p")
	writeFile(t, dir, "p.go", "edited since\n")

	for _, rev := range []string{gitOut(t, dir, "rev-parse", "HEAD"), gitOut(t, dir, "rev-parse", "HEAD^{tree}")} {
		got, whole, err := vcs.NewCLI().Blob(dir, rev, "p.go", 1<<10)
		if err != nil || !whole || string(got) != "package p\n\nvar s = `\n" {
			t.Errorf("Blob(%s) = %q, %v, %v; want the committed file, whole", rev, got, whole, err)
		}
	}
}

// A file over the bound is not read at all: it says so, and the caller lexes
// the hunk alone, as before (#800).
func TestCLI_BlobDoesNotReadPastItsBound_issue800(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "big.txt", strings.Repeat("x", 100))
	gitOut(t, dir, "add", "big.txt")
	gitOut(t, dir, "commit", "-m", "big")

	got, whole, err := vcs.NewCLI().Blob(dir, "HEAD", "big.txt", 10)
	if err != nil || whole || got != nil {
		t.Errorf("Blob over the bound = %q, %v, %v; want nothing, not whole", got, whole, err)
	}
}

// A path the revision does not have - a file added since - is an error that
// says which.
func TestCLI_BlobOfAMissingPathSaysWhich_issue800(t *testing.T) {
	dir := newRepo(t)
	if _, _, err := vcs.NewCLI().Blob(dir, "HEAD", "new.go", 10); err == nil || !strings.Contains(err.Error(), "new.go") {
		t.Errorf("Blob of a missing path: error = %v, want one naming it", err)
	}
}
