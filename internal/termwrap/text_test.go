package termwrap_test

import (
	"os/exec"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Text is a stream selection over the pane's own grid, which is what lets a
// drag copy the pane without the sidebar and file tree sharing its rows
// (#360). From (2,0) to (3,1) that is "CDEF" - the rest of the first row,
// trailing blanks trimmed - then "GHIJ".
func TestTerminal_TextTakesAStreamRunAndTrimsEachRow_issue360(t *testing.T) {
	term, err := termwrap.Start(20, 4, exec.Command("sh", "-c", `printf 'ABCDEF\nGHIJKL\n'; while :; do sleep 1; done`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()
	if frame := pump(t, term, "GHIJKL", 5*time.Second); frame == "" {
		t.Fatal("the child never painted")
	}

	if got, want := term.Text(2, 0, 3, 1), "CDEF\nGHIJ"; got != want {
		t.Errorf("Text(2,0,3,1) = %q, want %q", got, want)
	}
	if got, want := term.Text(1, 0, 3, 0), "BCD"; got != want {
		t.Errorf("a single-row run = %q, want %q", got, want)
	}
}

// A double-width grapheme occupies two cells, the second a placeholder with
// Width 0. Copying it twice would duplicate the character (#360).
func TestTerminal_TextDoesNotRepeatAWideGrapheme_issue360(t *testing.T) {
	term, err := termwrap.Start(20, 4, exec.Command("sh", "-c", `printf '你好Z\n'; while :; do sleep 1; done`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()
	if frame := pump(t, term, "Z", 5*time.Second); frame == "" {
		t.Fatal("the child never painted")
	}

	if got, want := term.Text(0, 0, 4, 0), "你好Z"; got != want {
		t.Errorf("Text over wide cells = %q, want %q", got, want)
	}
}

// The Fake reads a grid a test sets, so ui can assert what a drag copies
// without driving a real emulator (#360).
func TestFake_TextReadsTheGridItWasGiven_issue360(t *testing.T) {
	f := termwrap.NewFake("")
	f.Grid = []string{"hello", "world"}

	if got, want := f.Text(1, 0, 2, 1), "ello\nwor"; got != want {
		t.Errorf("Fake.Text = %q, want %q", got, want)
	}
	if got, want := f.Text(0, 0, 0, 0), "h"; got != want {
		t.Errorf("Fake.Text one cell = %q, want %q", got, want)
	}
}
