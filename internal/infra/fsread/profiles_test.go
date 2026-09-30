package fsread_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/fsread"
)

// The port's one method reads the profile at path with the module path of
// the go.mod at dir, which is what keys a Go profile's records to the
// repo-relative files a diff names (ADR 0001, migration step 5.3, #653).
func TestCoverageProfiles_readsAProfileAgainstTheModuleAtDir_issue653(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/m\n")
	mustWrite(t, filepath.Join(dir, "cover.out"), "mode: set\nexample.com/m/a.go:3.10,5.4 2 1\n")

	p, err := fsread.CoverageProfiles{}.Load(context.Background(), filepath.Join(dir, "cover.out"), dir)

	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if covered, known := p.Files["a.go"].Lines[3]; !known || !covered {
		t.Errorf("a.go:3 = (%v, %v), want a covered line keyed repo-relative", covered, known)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
