// Package agent describes the coding agents omatty can run. An agent is a
// command template plus a status adapter: how to start it, where it writes
// its transcript, which hook events it reports, and how to read one line of
// that transcript into omatty's neutral status vocabulary.
//
// Claude is the only profile today (#46). It exists as a profile rather than
// as the hardcoded default it was, so a second agent is a new file here and
// not a simultaneous edit to supervisor, watcher, paths and cmd.
//
// Nothing here starts a process. Command returns an argument list, not an
// *exec.Cmd, so the rule that only internal/supervisor runs a binary and only
// internal/detach holds it survives the seam (invariant 4). Nothing here can
// read a screen either: Status takes transcript bytes and hook payloads, and
// a Profile has no field a terminal could be handed through (invariant 2).
package agent

import (
	"fmt"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Profile is one agent. Every field is a pure function or a value, so a
// Profile is safe to copy and carries no lifecycle.
type Profile struct {
	// Name is what state.json records and what Lookup answers to.
	Name string
	// DefaultBin is the binary to run when the config file names none.
	DefaultBin string
	// Command is the argument list, bin included, for one session. resume is
	// true once the agent has written a transcript for the id: claude refuses
	// --session-id then, because the transcript itself is the claim (#36).
	Command func(bin, sessionID, dir string, resume bool, settingsFile string) []string
	// TranscriptPath is where the agent writes the session's JSONL.
	TranscriptPath func(home, dir, sessionID string) string
	// HookEvents is every event name the settings file must subscribe to. A
	// function value, not a slice, so it stays the same list the listener
	// maps rather than a copy of it (#78).
	HookEvents func() []string
	// RenderSettings is the settings-file content that makes the agent
	// report to `omatty hook`.
	RenderSettings func(binPath string, eventNames []string) ([]byte, error)
	// Status reads this agent's transcript lines and hook payloads into
	// omatty's neutral vocabulary. It is the half of a Profile the watcher
	// holds; the command template is the half only the supervisor needs.
	Status watcher.Adapter
}

// Lookup returns the profile a Session names. An empty name is claude: every
// session written before #46 has one, and an empty value that is derivable
// is what lets state.json stay at Version 1 (invariant 9).
//
//	p, err := agent.Lookup(sess.Agent)
func Lookup(name string) (Profile, error) {
	if name == "" || name == Claude().Name {
		return Claude(), nil
	}
	return Profile{}, fmt.Errorf("agent %q is not one omatty knows, want one of %s", name, strings.Join(Names(), ", "))
}

// Names is every profile omatty knows, for Lookup's error and for the config
// file's documentation.
func Names() []string { return []string{Claude().Name} }
