package crap

import "fmt"

// wrap names what the scorer was doing when something failed, so a gate step's
// output says which file rather than only that one broke.
func wrap(doing string, err error) error {
	return fmt.Errorf("crap: %s: %w", doing, err)
}
