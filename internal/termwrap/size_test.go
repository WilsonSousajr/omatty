package termwrap_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/creack/pty"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

func TestWindowSize_ReturnsColumnsThenRows_issue74(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Skip("no pty available:", err)
	}
	defer func() { _ = ptmx.Close(); _ = tty.Close() }()
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 140}); err != nil {
		t.Fatal(err)
	}

	w, h, err := termwrap.WindowSize(tty)

	if err != nil || w != 140 || h != 40 {
		t.Errorf("WindowSize = (%d, %d, %v), want (140, 40, nil): columns first", w, h, err)
	}
}

// Off a tty the caller must get an error to log, not a silent default.
func TestWindowSize_ErrorsOffATTY_issue74(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "not-a-tty"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if _, _, err := termwrap.WindowSize(f); err == nil {
		t.Error("WindowSize on a regular file returned nil error")
	}
}
