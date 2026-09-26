package forge

import (
	"strings"
	"time"
)

// NewCLIWithBin runs bin in place of gh, so a test can stand a script in.
func NewCLIWithBin(bin string) *CLI { return &CLI{bin: bin, timeout: listTimeout} }

// NewCLIWithTimeout is NewCLIWithBin with its own bound, so a test of a
// stalled gh takes milliseconds rather than the thirty seconds a real one
// is given (#356).
func NewCLIWithTimeout(bin string, d time.Duration) *CLI { return &CLI{bin: bin, timeout: d} }

// FinishedFieldsInclude reports whether the finished-pull-request field set asks
// gh for name. The request is half of what makes a field reach the fold, and a
// fold test cannot see it (#332).
func FinishedFieldsInclude(name string) bool {
	for _, f := range strings.Split(finishedFields, ",") {
		if f == name {
			return true
		}
	}
	return false
}
