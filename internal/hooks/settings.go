// Package hooks generates the settings file that makes Claude Code report
// status to omatty, and implements the `omatty hook` command that does the
// reporting.
package hooks

import "encoding/json"

// statusEvents are the hook events omatty listens to. PermissionRequest is a
// dedicated event, cleaner than parsing Notification; Notification still
// covers idle_prompt. Order is fixed so the output is stable.
var statusEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"PermissionRequest", "Notification", "Stop", "SessionEnd",
}

type handler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type group struct {
	Hooks []handler `json:"hooks"`
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
	h := handler{Type: "command", Command: binPath + " hook", Timeout: 5}
	events := make(map[string][]group, len(statusEvents))
	for _, name := range statusEvents {
		events[name] = []group{{Hooks: []handler{h}}}
	}
	return json.MarshalIndent(settings{Hooks: events}, "", "  ")
}
