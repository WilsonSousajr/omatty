// Package agent describes the coding agents omatty can run. An agent is a
// command template plus a status adapter: how to start it, where it writes
// its transcript, which hook events it reports, and how to read one line of
// that transcript into omatty's neutral status vocabulary.
//
// Claude was the first profile (#46) and codex the second (#152). Claude
// became a profile rather than the hardcoded default it was, so a second
// agent is a new catalog entry and not a simultaneous edit to the launcher,
// the watcher, paths and cmd. Each session's agent is resolved through an
// injected Catalog (#521).
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

import (
	"io"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/status"
)

// Profile is one agent. Every field is a pure function or a value, so a
// Profile is safe to copy and carries no lifecycle.
type Profile struct {
	// Name is what state.json records and what the catalog in cmd/omatty looks up.
	Name string
	// DefaultBin is the binary to run when the config file names none.
	DefaultBin string
	// Caps is what the agent offers, and so what omatty may claim about its
	// sessions: the tier is derived from it, never declared (#520).
	Caps Caps
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
	// RenderArgs is the alternative to RenderSettings for an agent that reads
	// no file omatty may write: its hooks travel as arguments, which the
	// launcher appends to Command's argv. Codex takes them as `-c` flags,
	// trust included (#152).
	RenderArgs func(binPath string, eventNames []string) ([]string, error)
	// Locate lists the conversations in the agent's store for dir begun at
	// or after since: how a Scanned identity is found (#523).
	Locate func(home, dir string, since time.Time) []string
	// ParseHook reads this agent's hook payload from the hook's stdin.
	ParseHook func(stdin io.Reader) (status.HookPayload, bool)
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

// CodexCommand is codex's argument list (#152). codex takes no id from
// omatty: a fresh start is the binary alone, and the id codex chooses comes
// back on its SessionStart hook (Reported, #523). A resume names that id,
// the row's Conversation, to `codex resume`. There is no settings file -
// codex reads hooks only from `-c` flags, which the launcher appends from
// the profile's RenderArgs, and which `codex resume` accepts after the id.
//
//	argv := agent.CodexCommand("codex", conversation, dir, true, "") // codex resume <id>
func CodexCommand(bin, sessionID, _ string, resume bool, _ string) []string {
	if resume {
		return []string{bin, "resume", sessionID}
	}
	return []string{bin}
}

// Generic is an agent declared in config.toml by its command alone (#525): it
// runs that command in the session's directory, with the configured binary
// in place of its first word, and nothing else. No session id, no resume, no
// settings file, no transcript - so its Caps are the zero value, the Process
// tier: omatty knows it runs and when it exits. It is how Aider, Goose,
// Crush, Droid and next month's agent run in omatty on day one.
//
//	p := agent.Generic("aider", []string{"aider", "--no-auto-commits"})
func Generic(name string, argv []string) Profile {
	args := append([]string(nil), argv[1:]...)
	return Profile{
		Name:       name,
		DefaultBin: argv[0],
		Command: func(bin, _, _ string, _ bool, _ string) []string {
			return append([]string{bin}, args...)
		},
	}
}

// KeepsTranscript reports whether omatty can read the agent's transcript:
// a path to find it and a parser to read it. A generic agent has neither
// (#525), so nothing may tail, name or resume it from one.
//
//	if !p.KeepsTranscript() { return } // Process tier
func (p Profile) KeepsTranscript() bool { return p.TranscriptPath != nil && p.Status != nil }
