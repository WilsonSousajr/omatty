// Package hooks generates the settings file that makes Claude Code report
// status to omatty, and implements the `omatty hook` command that does the
// reporting.
package hooks

import (
	"encoding/json"
	"strings"
)

// statusEvents are the hook events omatty listens to. PermissionRequest is a
// dedicated event, cleaner than parsing Notification; Notification still
// covers idle_prompt. Order is fixed so the output is stable.
var statusEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"PermissionRequest", "Notification", "Stop", "SessionEnd",
}

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

// Render returns the JSON for ~/.omatty/hooks.json: every status event runs
// `<binPath> hook`. binPath must be absolute — claude runs hooks with
// whatever PATH it inherited, which need not include omatty's directory.
//
//	content, _ := hooks.Render("/Users/w/go/bin/omatty")
func Render(binPath string) ([]byte, error) {
	h := hookCommand{Type: "command", Command: shellQuote(binPath) + " hook", Timeout: 5}
	events := make(map[string][]group, len(statusEvents))
	for _, name := range statusEvents {
		events[name] = []group{{Hooks: []hookCommand{h}}}
	}
	return json.MarshalIndent(settings{Hooks: events}, "", "  ")
}

// shellQuote wraps s in single quotes for a POSIX shell, escaping any single
// quote inside it. claude runs command hooks through a shell, so a path with
// a space or a metacharacter was split or expanded (issue #56).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
