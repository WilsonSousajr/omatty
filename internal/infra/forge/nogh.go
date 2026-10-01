package forge

import dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"

// NoGH is a missing gh: the answer a caller with no forge wired gives, the way
// a machine without gh answers, so it need not name the tool itself - only this
// package may (TestNoGhOutsideForge).
//
//	func noPRs(string) ([]forge.PR, error) { return nil, forge.NoGH() }
func NoGH() error { return &dforge.MissingToolError{Tool: "gh"} }
