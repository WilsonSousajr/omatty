package registry_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update rewrites golden files instead of comparing them. Goldens change only
// on purpose: go test ./internal/registry -run <Test> -update (#620).
var update = flag.Bool("update", false, "rewrite golden files")

// assertGolden fails when got differs from testdata/name, so a change to what
// omatty persists is always a visible diff in review rather than a silent one.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		writeGolden(t, path, got)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s (run with -update to create it): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the golden file; if the change is intended, rerun with -update\n--- got\n%s\n--- want\n%s",
			path, got, want)
	}
}

func writeGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, got, 0o600); err != nil {
		t.Fatal(err)
	}
}
