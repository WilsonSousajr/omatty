package app_test

import (
	"bytes"
	"flag"
	"github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/tui/app"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
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
func sceneDeps(st session.State, terms map[string]terminal.Terminal) app.Deps {
	d := baseDeps(st, terms)
	d.Clock = func() time.Time { return fixedNow }
	// A machine without gh, as every scene has always been drawn: the TUI's
	// own unwired default became "no forge" in migration step 5.9 (#653),
	// since it may no longer name a forge's CLI, and the goldens are pinned
	// on the answer a real machine gives, not on what an unwired model says.
	d.PRs = func(string) ([]forge.PR, error) { return nil, noGH }
	d.Issues = func(string) ([]forge.Issue, error) { return nil, noGH }
	noItem := func(string, int) (forge.Detail, error) { return forge.Detail{}, noGH }
	d.Item = app.ForgeItemFuncs{Issue: noItem, PR: noItem}
	return d
}

// sized delivers a window size and returns m, so a scene is one expression.
func sized(m *app.Model, w, h int) *app.Model {
	_, cmd := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	deliver(m, cmd)
	return m
}
