// Package termwrap is omatty's only route to an embedded terminal emulator.
//
// Invariant 4: bubbleterm is pre-1.0 and will break. It is imported in
// bubbleterm.go and nowhere else, so a breaking release touches one file.
package termwrap

import (
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

// Terminal is one embedded terminal running one process.
//
// View returns a plain string rather than a tea.View because bubbleterm sets
// no cursor, colors, or window title on the view it returns - only Content.
// If a later version does, widen this interface rather than leaking tea.View
// to callers.
type Terminal interface {
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	View() string
	SendInput(s string) tea.Cmd
	Resize(w, h int) tea.Cmd
	Focus()
	Blur()
	Focused() bool
	// Cursor is where the running application has left its cursor. The view
	// does not paint it - the emulator renders cell contents only - so the
	// caller draws it (issue #106).
	Cursor() Caret
	Close() error
}

// Caret is the emulated terminal's cursor: its cell in the emulator's own
// grid, whether the running application has it shown, and how it asked for it
// to be drawn. omatty draws no cursor of its own, so without this the caret in
// Claude's prompt is invisible (issue #106).
//
// X and Y are cells of the emulator's grid, not of the window; ui.PaneOrigin
// is what turns one into the other.
//
//	c := term.Cursor()
//	if c.Visible { x, y := ui.PaneOrigin(); draw(x+c.X, y+c.Y) }
//
// Color is deliberately absent. The emulator carries one (OSC 12) and
// tea.Cursor accepts one, but after an OSC 112 reset vt reports the *default*
// colour rather than nil, so forwarding it would repaint the host's cursor for
// every session that ever touched OSC 12 and never hand it back. Shape and
// Blink come from the same struct and are forwarded, so the gap is a decision
// rather than an oversight (#106).
type Caret struct {
	X, Y    int
	Visible bool
	Shape   tea.CursorShape
	Blink   bool
}

// Factory creates a Terminal running cmd. Injected so tests never spawn a
// real process.
type Factory func(w, h int, cmd *exec.Cmd) (Terminal, error)
