// The ways a forge answers "no" that the TUI turns into a note rather than an
// outage (#248, #449, #453, #460, #584). Moved here from internal/infra/forge
// in migration step 5.9 (Amendment 9, #653): they are vocabulary the TUI
// reads, and the TUI may not import infra.

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

// PlainHTTPError is a token omatty will not send: the remote is plain http,
// where anyone on the path, or an HTTP_PROXY, reads the headers. It stops the
// project with a note, as a missing tool does - no poll changes a remote's
// scheme, and an untyped refusal was asked again every poll (#584).
//
//	var plain *forge.PlainHTTPError
//	if errors.As(err, &plain) { note := plain.TokenEnv + " is sent only over https" }
type PlainHTTPError struct {
	// Host is the http host the token would have gone to.
	Host string
	// TokenEnv is the variable the token was borrowed from.
	TokenEnv string
}

// Error says what was refused and what would fix it.
func (e *PlainHTTPError) Error() string {
	return "forge: refusing to send " + e.TokenEnv + "'s token to " + e.Host + " over plain http; the remote must be https"
}

// ErrNoForge is the answer for a checkout omatty cannot map to a repository on
// a forge it reads - no remote, another host, or not a repository at all. It
// is quiet: the project simply has no pull requests or issues to show.
//
//	if errors.Is(err, forge.ErrNoForge) { /* stop polling this project */ }
var ErrNoForge = errors.New("forge: no repository on a forge omatty reads")

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

// ErrNoTracker is a forge that keeps no issues for this project: Bitbucket
// Cloud retired its issue tracker API (CHANGE-3071), and Bitbucket Data Center
// never had one - the issues are in Jira, which M16 leaves out. The tracker
// shows the project's pull requests and says where its issues are not.
//
//	if errors.Is(err, forge.ErrNoTracker) { /* show pull requests alone */ }
var ErrNoTracker = errors.New("forge: this forge keeps no issues for the project")
