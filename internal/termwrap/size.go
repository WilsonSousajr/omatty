package termwrap

import (
	"fmt"
	"os"

	"github.com/creack/pty"
)

// WindowSize returns the terminal attached to f in columns and rows, so the
// first PTY is born at the real pane size (issue #51). It lives here rather
// than in cmd so the query sits behind omatty's own terminal seam and inside
// the coverage gate (issue #74). Off a tty it returns an error the caller
// logs before falling back to a default.
//
//	w, h, err := termwrap.WindowSize(os.Stdout)
func WindowSize(f *os.File) (w, h int, err error) {
	rows, cols, err := pty.Getsize(f)
	if err != nil {
		return 0, 0, fmt.Errorf("termwrap: querying the size of %q: %w", f.Name(), err)
	}
	if cols == 0 || rows == 0 {
		return 0, 0, fmt.Errorf("termwrap: %q reports a %dx%d terminal, want both non-zero", f.Name(), cols, rows)
	}
	return cols, rows, nil
}
