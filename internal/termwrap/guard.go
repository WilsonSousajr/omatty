package termwrap

import (
	"fmt"
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
// Every method of Terminal is wrapped, not just the ones on the render path.
// Guarding only View and Update left Resize and Close reaching the emulator
// bare, and M4 moved both onto the hot path: Resize now fires on every window
// change including behind a modal (#95), and Close became a mid-run call when
// a session can be archived (#40). A panic in either took the whole app down.
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

// guarded runs fn unless this terminal has already panicked, containing any
// panic it raises to this one session. It is the single recover site: three
// hand-copied versions of this block were what let the interface grow past
// them unnoticed.
//
// what completes the sentence "terminal panicked ...". fn writes its result
// into a named return of the calling method, so a panic leaves that return at
// whatever the method set before calling - the zero value, or crashFrame.
func (g *Guard) guarded(what string, fn func()) {
	if g.Panicked {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			g.Panicked = true
			slog.Error("terminal panicked "+what, "panic", r)
		}
	}()
	fn()
}

// View renders the wrapped terminal, substituting an error frame if it panics.
func (g *Guard) View() (frame string) {
	frame = crashFrame
	g.guarded("while rendering", func() { frame = g.Terminal.View() })
	return frame
}

// Cursor reads the wrapped terminal's caret. A crashed session shows the crash
// frame, which has no caret of its own, so the zero Caret is correct (#106).
func (g *Guard) Cursor() (c Caret) {
	g.guarded("while reading its cursor", func() { c = g.Terminal.Cursor() })
	return c
}

// Update forwards the message, swallowing a panic from the emulator.
func (g *Guard) Update(msg tea.Msg) (cmd tea.Cmd) {
	g.guarded("while updating", func() { cmd = g.Terminal.Update(msg) })
	return cmd
}

// Init starts the wrapped terminal.
func (g *Guard) Init() (cmd tea.Cmd) {
	g.guarded("while starting", func() { cmd = g.Terminal.Init() })
	return cmd
}

// SendInput writes s to the pty.
func (g *Guard) SendInput(s string) (cmd tea.Cmd) {
	g.guarded("while sending input", func() { cmd = g.Terminal.SendInput(s) })
	return cmd
}

// Resize reflows the emulator. This runs on every window change, including
// behind an open modal (#95), so it is the method most likely to reach a
// pre-1.0 reflow bug.
func (g *Guard) Resize(w, h int) (cmd tea.Cmd) {
	g.guarded("while resizing", func() { cmd = g.Terminal.Resize(w, h) })
	return cmd
}

// Focus gives the wrapped terminal the keyboard.
func (g *Guard) Focus() { g.guarded("while focusing", func() { g.Terminal.Focus() }) }

// Blur takes the keyboard away from the wrapped terminal.
func (g *Guard) Blur() { g.guarded("while blurring", func() { g.Terminal.Blur() }) }

// Focused reports whether the wrapped terminal holds the keyboard. A crashed
// terminal holds nothing.
func (g *Guard) Focused() (focused bool) {
	g.guarded("while reading its focus", func() { focused = g.Terminal.Focused() })
	return focused
}

// Close releases the pty.
//
// Unlike every other method this still runs after a panic: leaking the pty and
// the claude process behind it is worse than a second panic, which the recover
// here contains anyway. The error is returned rather than swallowed so the
// caller can say which session failed to close (#40).
func (g *Guard) Close() (err error) {
	defer func() {
		if r := recover(); r != nil {
			g.Panicked = true
			slog.Error("terminal panicked while closing", "panic", r)
			err = fmt.Errorf("termwrap: terminal panicked while closing: %v", r)
		}
	}()
	return g.Terminal.Close()
}
