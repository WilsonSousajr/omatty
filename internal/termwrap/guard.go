package termwrap

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
)

// crashFrame replaces a crashed session's output. It names the recovery key
// so the operator is not left staring at a dead pane.
const crashFrame = "x this session's terminal crashed - press ctrl+o r to restart it"

// Guard wraps a Terminal so a panic inside the emulator takes down one
// session's widget rather than the whole app (invariant 6). bubbleterm is
// pre-1.0; this is the containment for the bugs that implies.
//
//	term = termwrap.NewGuard(term)
type Guard struct {
	Terminal
	// Panicked reports whether the wrapped terminal has ever panicked. A
	// guarded terminal is not retried; the session stays marked crashed.
	Panicked bool
}

// NewGuard wraps t.
func NewGuard(t Terminal) *Guard { return &Guard{Terminal: t} }

// View renders the wrapped terminal, substituting an error frame if it panics.
func (g *Guard) View() (frame string) {
	if g.Panicked {
		return crashFrame
	}
	defer func() {
		if r := recover(); r != nil {
			g.Panicked = true
			slog.Error("terminal panicked while rendering", "panic", r)
			frame = crashFrame
		}
	}()
	return g.Terminal.View()
}

// Update forwards the message, swallowing a panic from the emulator.
func (g *Guard) Update(msg tea.Msg) (cmd tea.Cmd) {
	if g.Panicked {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			g.Panicked = true
			slog.Error("terminal panicked while updating", "panic", r)
			cmd = nil
		}
	}()
	return g.Terminal.Update(msg)
}
