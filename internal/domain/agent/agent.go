// Package agent describes the coding agents omatty can run. An agent is a
// command template plus a status adapter: how to start it, where it writes
// its transcript, which hook events it reports, and how to read one line of
// that transcript into omatty's neutral status vocabulary.
//
// Claude is the only profile today (#46). It exists as a profile rather than
// as the hardcoded default it was, so a second agent is a new catalog entry and
// not a simultaneous edit to supervisor, watcher, paths and cmd.
//
// The catalog - which profiles exist, and the implementations each one
// carries - is composed in cmd/omatty (ADR 0001, migration step 5.2b,
// #653): a profile names a transcript parser, a settings renderer and a
// path function, and those live in layers this package may not import.
//
// Nothing here starts a process. Command returns an argument list, not an
// *exec.Cmd, so the rule that only internal/termwrap spawns an agent - from the
// session.Launch the session service builds - and only internal/infra/detach
// holds it survives the seam (invariant 4). Nothing here can
// read a screen either: Status takes transcript bytes and hook payloads, and
// a Profile has no field a terminal could be handed through (invariant 2).
package agent

import "github.com/WilsonSousajr/omatty/internal/domain/status"

// Profile is one agent. Every field is a pure function or a value, so a
// Profile is safe to copy and carries no lifecycle.
type Profile struct {
	// Name is what state.json records and what the catalog in cmd/omatty looks up.
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
	// holds; the command template is the half only the launcher needs.
	Status status.Adapter
}

// ClaudeCommand is claude's argument list. A session that has never spoken
// starts with --session-id, which lets omatty choose the uuid and so know
// the transcript path (invariant 2). Once a transcript exists claude refuses
// that flag - "Session ID <uuid> is already in use" - because the transcript
// itself is the claim, so it is resumed instead (#36). Either way --settings
// names omatty's own file, never the user's (invariant 3).
//
//	argv := agent.ClaudeCommand("claude", id, dir, false, "/h/.omatty/hooks.json")
func ClaudeCommand(bin, sessionID, _ string, resume bool, settingsFile string) []string {
	flag := "--session-id"
	if resume {
		flag = "--resume"
	}
	return []string{bin, flag, sessionID, "--settings", settingsFile}
}
