package forge

import "errors"

// MissingToolError is a forge omatty cannot reach from this machine: its CLI
// is not on PATH and, once the forge has a REST fallback, the token that would
// stand in for it is unset too. Nothing to ask, and not a failure to report
// again on every poll - the answer the gate learned from a missing tool (#248).
//
//	var missing *forge.MissingToolError
//	if errors.As(err, &missing) { note := missing.Tool + " is not installed" }
type MissingToolError struct {
	// Tool is the forge's CLI, "gh" or "glab"; empty for a forge with none.
	Tool string
	// TokenEnv is the variable the REST fallback reads. Empty until the forge
	// has one, so the message never advises a variable nothing would read.
	TokenEnv string
}

// Error names both halves of the fix, "install X or set Y" (#449).
func (e *MissingToolError) Error() string {
	switch {
	case e.Tool == "":
		return "forge: " + e.TokenEnv + " is unset"
	case e.TokenEnv == "":
		return "forge: " + e.Tool + " not found on PATH"
	}
	return "forge: " + e.Tool + " not found on PATH and " + e.TokenEnv + " is unset"
}

// ErrNoForge is the answer for a checkout omatty cannot map to a repository on
// a forge it reads - no remote, another host, or not a repository at all. It
// is quiet: the project simply has no pull requests or issues to show.
//
//	if errors.Is(err, forge.ErrNoForge) { /* stop polling this project */ }
var ErrNoForge = errors.New("forge: no repository on a forge omatty reads")

// NoGH is a missing gh: the answer a caller with no forge wired gives, the way
// a machine without gh answers, so it need not name the tool itself - only this
// package may (TestNoGhOutsideForge).
//
//	func noPRs(string) ([]forge.PR, error) { return nil, forge.NoGH() }
func NoGH() error { return &MissingToolError{Tool: "gh"} }
