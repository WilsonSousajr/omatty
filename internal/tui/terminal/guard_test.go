package terminal_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// PanicTerminal is a named fake whose every method panics, standing in for a
// bug in the pre-1.0 emulator (invariant 6).
//
// Every method, not just View and Update: Guard covered only those two, so the
// other seven reached bubbleterm bare - and M4 moved Resize and Close onto the
// hot path (#95, #40). A fake that panicked in two places could not have shown
// that (#112).
type PanicTerminal struct{ *terminal.Fake }

func (p *PanicTerminal) View() string             { panic("emulator exploded") }
func (p *PanicTerminal) Update(tea.Msg) tea.Cmd   { panic("emulator exploded") }
func (p *PanicTerminal) Cursor() terminal.Caret   { panic("emulator exploded") }
func (p *PanicTerminal) Init() tea.Cmd            { panic("emulator exploded") }
func (p *PanicTerminal) SendInput(string) tea.Cmd { panic("emulator exploded") }
func (p *PanicTerminal) Resize(int, int) tea.Cmd  { panic("emulator exploded") }
func (p *PanicTerminal) Focus()                   { panic("emulator exploded") }
func (p *PanicTerminal) Blur()                    { panic("emulator exploded") }
func (p *PanicTerminal) Focused() bool            { panic("emulator exploded") }
func (p *PanicTerminal) Close() error             { panic("emulator exploded") }

func (p *PanicTerminal) Text(_, _, _, _ int) string { panic("emulator exploded") }
func (p *PanicTerminal) Repaint() tea.Cmd           { panic("emulator exploded") }

func TestGuard_ViewPanicBecomesAnErrorFrame(t *testing.T) {
	g := terminal.NewGuard(&PanicTerminal{Fake: terminal.NewFake("")}, "ctrl+o r")

	got := g.View()

	if !g.Panicked {
		t.Error("Panicked = false after a panicking View, want true")
	}
	if !strings.Contains(got, "crashed") {
		t.Errorf("View() = %q, want a frame mentioning the crash", got)
	}
}

func TestGuard_UpdatePanicIsContained(t *testing.T) {
	g := terminal.NewGuard(&PanicTerminal{Fake: terminal.NewFake("")}, "ctrl+o r")

	if cmd := g.Update(tea.WindowSizeMsg{}); cmd != nil {
		t.Errorf("Update() = %v after a panic, want nil", cmd)
	}
	if !g.Panicked {
		t.Error("Panicked = false after a panicking Update, want true")
	}
}

// Once a terminal has panicked it is not asked again: a repeatedly panicking
// emulator would otherwise burn a recover on every frame.
func TestGuard_StaysCrashedWithoutReenteringTheTerminal(t *testing.T) {
	g := terminal.NewGuard(&PanicTerminal{Fake: terminal.NewFake("")}, "ctrl+o r")
	g.View()

	if got := g.View(); !strings.Contains(got, "crashed") {
		t.Errorf("View() = %q on a crashed terminal, want the crash frame", got)
	}
	if cmd := g.Update(tea.WindowSizeMsg{}); cmd != nil {
		t.Errorf("Update() = %v on a crashed terminal, want nil", cmd)
	}
}

func TestGuard_HealthyTerminalPassesThrough(t *testing.T) {
	f := terminal.NewFake("hello")
	g := terminal.NewGuard(f, "ctrl+o r")

	if got := g.View(); got != "hello" {
		t.Errorf("View() = %q, want %q", got, "hello")
	}
	g.Update(tea.WindowSizeMsg{})
	g.SendInput("typed")
	g.Resize(80, 24)

	if g.Panicked {
		t.Error("Panicked = true for a healthy terminal, want false")
	}
	if len(f.Msgs) != 1 || len(f.Sent) != 1 || f.Width != 80 {
		t.Errorf("guard did not forward to the wrapped terminal: msgs=%d sent=%v size=%dx%d",
			len(f.Msgs), f.Sent, f.Width, f.Height)
	}
}

var _ terminal.Terminal = (*terminal.Guard)(nil)

// Invariant 6 covers the whole interface, not the render path. Resize runs on
// every window change including behind a modal (#95) and Close became a
// mid-run call when a session can be archived (#40), so a panic in either used
// to take the app down with it (#112).
func TestGuard_ContainsAPanicFromEveryMethod_issue112(t *testing.T) {
	for name, call := range map[string]func(*terminal.Guard){
		"Init":      func(g *terminal.Guard) { g.Init() },
		"View":      func(g *terminal.Guard) { g.View() },
		"Update":    func(g *terminal.Guard) { g.Update(tea.WindowSizeMsg{}) },
		"Cursor":    func(g *terminal.Guard) { g.Cursor() },
		"SendInput": func(g *terminal.Guard) { g.SendInput("x") },
		"Resize":    func(g *terminal.Guard) { g.Resize(10, 10) },
		"Focus":     func(g *terminal.Guard) { g.Focus() },
		"Blur":      func(g *terminal.Guard) { g.Blur() },
		"Focused":   func(g *terminal.Guard) { g.Focused() },
		"Close":     func(g *terminal.Guard) { _ = g.Close() },
		"Text":      func(g *terminal.Guard) { g.Text(0, 0, 1, 1) },
		"Repaint":   func(g *terminal.Guard) { g.Repaint() },
	} {
		t.Run(name, func(t *testing.T) {
			g := terminal.NewGuard(&PanicTerminal{Fake: terminal.NewFake("")}, "ctrl+o r")

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s let a panic escape the guard: %v", name, r)
				}
			}()
			call(g)

			if !g.Panicked {
				t.Errorf("Panicked = false after a panicking %s, want true", name)
			}
		})
	}
}

// Regression, issue #118: Guard.Cursor had no test at all. Replacing its body
// with `return Caret{}` - which hides the caret for every real session, since
// run.go wraps them all - left the whole suite green, i.e. reproduced #106
// with nothing failing.
func TestGuard_CursorReportsTheWrappedTerminalsCaret_issue118(t *testing.T) {
	f := terminal.NewFake("")
	f.Caret = terminal.Caret{X: 4, Y: 2, Visible: true}
	g := terminal.NewGuard(f, "ctrl+o r")

	got := g.Cursor()

	if got != f.Caret {
		t.Errorf("Cursor() = %+v, want the wrapped terminal's %+v", got, f.Caret)
	}
}

// A crashed terminal shows the crash frame, which has no caret of its own.
func TestGuard_CrashedTerminalReportsNoCaret_issue118(t *testing.T) {
	g := terminal.NewGuard(&PanicTerminal{Fake: terminal.NewFake("")}, "ctrl+o r")
	g.View() // crash it

	if got := g.Cursor(); got != (terminal.Caret{}) {
		t.Errorf("Cursor() = %+v on a crashed terminal, want the zero Caret", got)
	}
}

// Close is the one method that still runs after a panic: leaking the pty and
// the claude process behind it is worse than a second panic, which the recover
// contains anyway. It reports the failure rather than swallowing it (#40).
func TestGuard_CloseStillRunsOnACrashedTerminal_issue112(t *testing.T) {
	g := terminal.NewGuard(&PanicTerminal{Fake: terminal.NewFake("")}, "ctrl+o r")
	g.View() // crash it

	if err := g.Close(); err == nil {
		t.Error("Close() = nil on a terminal that panics while closing, want an error naming it")
	}
}

// The crash frame names the key it was given, because the leader is
// configurable and termwrap cannot know it (#44).
func TestGuard_CrashFrameNamesTheRestartKeyItWasGiven_issue44(t *testing.T) {
	g := terminal.NewGuard(&PanicTerminal{Fake: terminal.NewFake("")}, "ctrl+a r")
	if frame := g.View(); !strings.Contains(frame, "press ctrl+a r to restart") {
		t.Errorf("crash frame %q does not name ctrl+a r", frame)
	}
}
