package termwrap

import (
	"fmt"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"github.com/taigrr/bubbleterm"
	"github.com/taigrr/bubbleterm/emulator"
)

// bubble adapts bubbleterm.Model to Terminal. It is unexported: callers get
// the interface from Start, never the concrete type.
type bubble struct{ m *bubbleterm.Model }

// Start launches cmd inside a w by h embedded terminal.
//
//	term, err := termwrap.Start(80, 24, exec.Command("claude", "--session-id", id))
func Start(w, h int, cmd *exec.Cmd) (Terminal, error) {
	m, err := bubbleterm.NewWithCommand(w, h, cmd)
	if err != nil {
		return nil, fmt.Errorf("termwrap: starting %q in a %dx%d terminal: %w", cmd.Path, w, h, err)
	}
	return &bubble{m: m}, nil
}

func (b *bubble) Init() tea.Cmd              { return b.m.Init() }
func (b *bubble) SendInput(s string) tea.Cmd { return b.m.SendInput(s) }
func (b *bubble) Resize(w, h int) tea.Cmd    { return b.m.Resize(w, h) }
func (b *bubble) Focus()                     { b.m.Focus() }
func (b *bubble) Blur()                      { b.m.Blur() }
func (b *bubble) Focused() bool              { return b.m.Focused() }
func (b *bubble) Close() error               { return b.m.Close() }

// Cursor reads the cursor straight off the emulator. bubbleterm's own view
// carries none, and the rendered grid does not paint the cell, so this is the
// only route to the caret in Claude's prompt (issue #106).
func (b *bubble) Cursor() Caret {
	emu := b.m.GetEmulator()
	pos, visible := emu.Cursor()
	look := emu.CursorAppearance()
	return Caret{X: pos.X, Y: pos.Y, Visible: visible, Shape: caretShape(look.Style), Blink: look.Blink}
}

// caretShape maps the emulator's cursor style to bubbletea's. An explicit
// switch rather than an int cast, so a change to either iota order is a
// compile error and not a silently wrong shape.
func caretShape(s emulator.CursorStyle) tea.CursorShape {
	switch s {
	case emulator.CursorUnderline:
		return tea.CursorUnderline
	case emulator.CursorBar:
		return tea.CursorBar
	default:
		return tea.CursorBlock
	}
}

// View returns the rendered cell grid. tea.View exposes no String method;
// Content is the field holding the styled screen text.
func (b *bubble) View() string { return b.m.View().Content }

// Update folds bubbleterm's returned model back in, so callers keep a stable
// Terminal reference across updates.
func (b *bubble) Update(msg tea.Msg) tea.Cmd {
	next, cmd := b.m.Update(msg)
	if m, ok := next.(*bubbleterm.Model); ok {
		b.m = m
	}
	return cmd
}
