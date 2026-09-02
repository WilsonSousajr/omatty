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
	Close() error
}

// Factory creates a Terminal running cmd. Injected so tests never spawn a
// real process.
type Factory func(w, h int, cmd *exec.Cmd) (Terminal, error)
