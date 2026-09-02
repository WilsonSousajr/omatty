package termwrap_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// PanicTerminal is a named fake whose View and Update always panic, standing
// in for a bug in the pre-1.0 emulator (invariant 6).
type PanicTerminal struct{ *termwrap.Fake }

func (p *PanicTerminal) View() string           { panic("emulator exploded") }
func (p *PanicTerminal) Update(tea.Msg) tea.Cmd { panic("emulator exploded") }

func TestGuard_ViewPanicBecomesAnErrorFrame(t *testing.T) {
	g := termwrap.NewGuard(&PanicTerminal{Fake: termwrap.NewFake("")})

	got := g.View()

	if !g.Panicked {
		t.Error("Panicked = false after a panicking View, want true")
	}
	if !strings.Contains(got, "crashed") {
		t.Errorf("View() = %q, want a frame mentioning the crash", got)
	}
}

func TestGuard_UpdatePanicIsContained(t *testing.T) {
	g := termwrap.NewGuard(&PanicTerminal{Fake: termwrap.NewFake("")})

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
	g := termwrap.NewGuard(&PanicTerminal{Fake: termwrap.NewFake("")})
	g.View()

	if got := g.View(); !strings.Contains(got, "crashed") {
		t.Errorf("View() = %q on a crashed terminal, want the crash frame", got)
	}
	if cmd := g.Update(tea.WindowSizeMsg{}); cmd != nil {
		t.Errorf("Update() = %v on a crashed terminal, want nil", cmd)
	}
}

func TestGuard_HealthyTerminalPassesThrough(t *testing.T) {
	f := termwrap.NewFake("hello")
	g := termwrap.NewGuard(f)

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

var _ termwrap.Terminal = (*termwrap.Guard)(nil)
