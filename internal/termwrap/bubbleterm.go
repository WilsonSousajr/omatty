package termwrap

import (
	"fmt"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"github.com/taigrr/bubbleterm"
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
