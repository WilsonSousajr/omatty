package forge

import "time"

// NewCLIWithBin runs bin in place of gh, so a test can stand a script in.
func NewCLIWithBin(bin string) *CLI { return &CLI{bin: bin, timeout: listTimeout} }

// NewCLIWithTimeout is NewCLIWithBin with its own bound, so a test of a
// stalled gh takes milliseconds rather than the thirty seconds a real one
// is given (#356).
func NewCLIWithTimeout(bin string, d time.Duration) *CLI { return &CLI{bin: bin, timeout: d} }
