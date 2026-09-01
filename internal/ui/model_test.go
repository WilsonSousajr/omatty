package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func noCreate(string, string, string) error { return nil }

func fakeTerms(t *testing.T) (map[string]termwrap.Terminal, map[string]*termwrap.Fake) {
	t.Helper()
	fakes := map[string]*termwrap.Fake{
		"s1": termwrap.NewFake("session one"),
		"s2": termwrap.NewFake("session two"),
		"s3": termwrap.NewFake("session three"),
	}
	terms := make(map[string]termwrap.Terminal, len(fakes))
	for id, f := range fakes {
		terms[id] = f
	}
	return terms, fakes
}

func modelWithFakes(t *testing.T) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	return ui.NewModel(twoProjectState(), terms, noCreate), fakes
}

func press(m *ui.Model, k tea.KeyPressMsg) { m.Update(k) }

func key(r rune) tea.KeyPressMsg     { return tea.KeyPressMsg{Code: r, Text: string(r)} }
func ctrl(r rune) tea.KeyPressMsg    { return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl} }
func special(c rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: c} }

// Invariant 1, end to end: a Claude binding reaches the PTY untouched. It
// must arrive as the message itself, because bubbleterm does its own
// key-to-escape translation - forwarding a rendered string would type the
// literal text "esc" into Claude.
func TestModel_escReachesTheFocusedTerminalAsAMessage(t *testing.T) {
	m, fakes := modelWithFakes(t)

	press(m, special(tea.KeyEscape))

	if len(fakes["s1"].Msgs) != 1 {
		t.Fatalf("focused terminal received %d messages, want 1", len(fakes["s1"].Msgs))
	}
	if len(fakes["s1"].Sent) != 0 {
		t.Errorf("focused terminal got SendInput %v; keystrokes must go through Update", fakes["s1"].Sent)
	}
	if len(fakes["s2"].Msgs) != 0 {
		t.Errorf("unfocused terminal received %v, want nothing", fakes["s2"].Msgs)
	}
}

func TestModel_claudeBindingsAllReachTheTerminal(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{
		special(tea.KeyEscape),
		special(tea.KeyEnter),
		{Code: tea.KeyTab, Mod: tea.ModShift},
		ctrl('r'),
		ctrl('c'),
		key('a'),
	} {
		t.Run(k.Keystroke(), func(t *testing.T) {
			m, fakes := modelWithFakes(t)
			press(m, k)
			if len(fakes["s1"].Msgs) != 1 {
				t.Errorf("%q did not reach the terminal", k.Keystroke())
			}
		})
	}
}

func TestModel_leaderThenJSwitchesSessionWithoutTouchingThePTY(t *testing.T) {
	m, fakes := modelWithFakes(t)

	press(m, ctrl('o'))
	press(m, key('j'))

	if got := m.Focused(); got != "s2" {
		t.Errorf("Focused() = %q, want %q after ctrl+o j", got, "s2")
	}
	for id, f := range fakes {
		if len(f.Msgs) != 0 || len(f.Sent) != 0 {
			t.Errorf("terminal %s received %v/%v; the leader must not leak", id, f.Msgs, f.Sent)
		}
	}
}

func TestModel_leaderThenKMovesBack(t *testing.T) {
	m, _ := modelWithFakes(t)

	press(m, ctrl('o'))
	press(m, key('j'))
	press(m, ctrl('o'))
	press(m, key('k'))

	if got := m.Focused(); got != "s1" {
		t.Errorf("Focused() = %q, want %q", got, "s1")
	}
}

func TestModel_leaderThenQQuits(t *testing.T) {
	m, _ := modelWithFakes(t)

	press(m, ctrl('o'))
	_, cmd := m.Update(key('q'))

	if cmd == nil {
		t.Fatal("ctrl+o q returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("command produced %T, want tea.QuitMsg", cmd())
	}
}

func TestModel_ViewShowsEveryProjectAndTheFocusedSession(t *testing.T) {
	m, _ := modelWithFakes(t)

	got := m.View().Content

	for _, want := range []string{"omatty", "api-svc", "main", "parser-fix", "session one"} {
		if !strings.Contains(got, want) {
			t.Errorf("View() does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "session two") {
		t.Errorf("View() shows an unfocused session's terminal:\n%s", got)
	}
}

func TestModel_WindowResizeGivesTheTerminalWhatThePanesLeave(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	f := fakes["s1"]
	if f.Width == 0 || f.Height == 0 {
		t.Fatalf("focused terminal size = %dx%d, want a propagated resize", f.Width, f.Height)
	}
	if f.Width >= 120 {
		t.Errorf("terminal width %d, want less than the window's 120 (sidebar and diff take space)", f.Width)
	}
	if f.Height != 40 {
		t.Errorf("terminal height %d, want the full 40", f.Height)
	}
}

func TestModel_NoSessionsRendersWithoutPanicking(t *testing.T) {
	m := ui.NewModel(emptyState(), map[string]termwrap.Terminal{}, noCreate)

	press(m, special(tea.KeyEscape))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if got := m.Focused(); got != "" {
		t.Errorf("Focused() = %q on an empty registry, want \"\"", got)
	}
	if m.View().Content == "" {
		t.Error("View() is empty on an empty registry, want at least a hint")
	}
}
