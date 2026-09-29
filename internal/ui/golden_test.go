package ui_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// update rewrites golden files instead of comparing them. Goldens change only
// on purpose: go test ./internal/ui -run <Test> -update (#620).
var update = flag.Bool("update", false, "rewrite golden files")

// assertGolden fails when got differs from testdata/name, so a change to what
// omatty draws is always a visible diff in review rather than a silent one.
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

// sceneDeps is baseDeps with the clock pinned. baseDeps leaves the wall clock
// in, and a card's age or a spinner frame read from it would make every
// golden fail a minute after it was written.
func sceneDeps(st registry.State, terms map[string]termwrap.Terminal) ui.Deps {
	d := baseDeps(st, terms)
	d.Clock = func() time.Time { return fixedNow }
	return d
}

// sized delivers a window size and returns m, so a scene is one expression.
func sized(m *ui.Model, w, h int) *ui.Model {
	_, cmd := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	deliver(m, cmd)
	return m
}
