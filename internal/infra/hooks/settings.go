// Package hooks generates the settings file that makes Claude Code report
// status to omatty, and implements the `omatty hook` command that does the
// reporting.
package hooks

import (
	"encoding/json"
	"strings"
)

type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type group struct {
	Hooks []hookCommand `json:"hooks"`
}

type settings struct {
	Hooks map[string][]group `json:"hooks"`
}

// Render returns the JSON for ~/.omatty/hooks.json: each named event runs
// `<binPath> hook`. binPath must be absolute — claude runs hooks with
// whatever PATH it inherited, which need not include omatty's directory. The
// event names come from the listener that consumes them (issue #78).
//
//	content, _ := hooks.Render("/Users/w/go/bin/omatty", watcher.HookEventNames())
func Render(binPath string, eventNames []string) ([]byte, error) {
	h := hookCommand{Type: "command", Command: hookLine(binPath), Timeout: 5}
	events := make(map[string][]group, len(eventNames))
	for _, name := range eventNames {
		events[name] = []group{{Hooks: []hookCommand{h}}}
	}
	return json.MarshalIndent(settings{Hooks: events}, "", "  ")
}

// hookLine is the shell command claude runs for one event, written so that a
// binary which is no longer there is also a silent success.
//
// Invariant 11 says a hook must never block or fail claude, and `omatty hook`
// keeps that once it runs. The command around it did not. claude reads
// --settings once, at startup, so a session keeps the absolute path of the
// omatty that launched it for as long as it lives; reinstalling elsewhere
// (go install -> brew, a brew upgrade to a new Cellar path, go clean) removes
// that file. `sh` then exited 127 with "No such file or directory" on stderr
// for every hook event, and claude showed it on every tool call in every
// running session (#380).
//
// `|| true` makes the exit 0. `2>/dev/null` is what silences sh's own message,
// and it costs nothing: invariant 11 already requires `omatty hook` to write
// nothing to stdout or stderr in every case - socket missing, connection
// refused, malformed JSON - so there is no diagnostic here to lose. claude's
// stdin payload simply goes unread, which is the same as it was when sh failed.
func hookLine(binPath string) string {
	return shellQuote(binPath) + " hook 2>/dev/null || true"
}

// shellQuote wraps s in single quotes for a POSIX shell, escaping any single
// quote inside it. claude runs command hooks through a shell, so a path with
// a space or a metacharacter was split or expanded (issue #56).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
