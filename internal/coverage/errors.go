package coverage

import "fmt"

// maxLineBytes bounds one profile line. Generous: an lcov SF: record carries a
// whole absolute path, and a Go record a whole import path.
const maxLineBytes = 1 << 20

// wrap names what omatty was doing when a read failed, so the log says which
// profile rather than only that one broke.
func wrap(doing string, err error) error {
	return fmt.Errorf("coverage: %s: %w", doing, err)
}
