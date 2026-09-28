package forge

import (
	"errors"
	"fmt"
)

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
	// NoLoginFor is the host a Tool on PATH holds no login for: tea reads only
	// through a login, so it is installed and still no way in (#586).
	NoLoginFor string
}

// Error names both halves of the fix, "install X or set Y" (#449).
func (e *MissingToolError) Error() string {
	tool := e.Tool + " not found on PATH"
	if e.NoLoginFor != "" {
		tool = e.Tool + " has no login for " + e.NoLoginFor
	}
	switch {
	case e.Tool == "":
		return "forge: " + e.TokenEnv + " is unset"
	case e.TokenEnv == "":
		return "forge: " + tool
	}
	return "forge: " + tool + " and " + e.TokenEnv + " is unset"
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

// AuthError is a forge that refused the token omatty borrowed from the
// environment: 401, a 403 that is not a rate limit, or Azure's sign-in page in
// place of JSON (#453). It is a note the operator can act on - fix or replace
// the token - rather than an outage, and it holds no field that could carry
// the token itself.
//
//	var refused *forge.AuthError
//	if errors.As(err, &refused) { note := refused.Host + " refused " + refused.TokenEnv }
type AuthError struct {
	// Host is the forge that refused, with its port when it has one.
	Host string
	// TokenEnv names the credential: the variable the token was borrowed
	// from, or a CLI's own login.
	TokenEnv string
	// Status is the HTTP status the refusal came with.
	Status int
}

// Error names the host, the credential and the status: the three things the
// operator needs to find and replace the right token.
func (e *AuthError) Error() string {
	return fmt.Sprintf("forge: %s refused %s (%d)", e.Host, e.TokenEnv, e.Status)
}
