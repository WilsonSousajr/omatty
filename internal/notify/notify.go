// Package notify posts desktop notifications when a session needs attention
// while omatty is in the background.
package notify

import (
	"os/exec"
	"strings"
)

// Notifier posts a desktop notification. Fake it in tests; do not shell out.
type Notifier interface {
	Notify(title, body string) error
}

// Osascript posts via macOS's osascript. On other platforms it is a no-op
// wrapper the caller can swap out.
type Osascript struct{}

// Notify runs `osascript -e 'display notification ...'`.
//
//	notify.Osascript{}.Notify("omatty", "parser-fix needs you")
func (Osascript) Notify(title, body string) error {
	argv := OsascriptArgv(title, body)
	return exec.Command(argv[0], argv[1:]...).Run() //nolint:gosec // fixed argv, escaped strings
}

// OsascriptArgv builds the command, escaping both strings for an AppleScript
// double-quoted literal (backslash first, then quote). Exposed so a test can
// assert the escaping without spawning a process.
func OsascriptArgv(title, body string) []string {
	script := "display notification \"" + escape(body) + "\" with title \"" + escape(title) + "\""
	return []string{"osascript", "-e", script}
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// Fake records notifications for tests.
type Fake struct {
	Sent []Note
}

// Note is one recorded notification.
type Note struct{ Title, Body string }

// Notify records the notification.
func (f *Fake) Notify(title, body string) error {
	f.Sent = append(f.Sent, Note{Title: title, Body: body})
	return nil
}
