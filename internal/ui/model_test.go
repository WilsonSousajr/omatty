package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func noCreate(_, title, branch string) (registry.Session, error) {
	return registry.Session{ID: "created", Title: title, Branch: branch}, nil
}

func noStart(registry.Session, int, int) (termwrap.Terminal, error) { return termwrap.NewFake(""), nil }

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

// baseDeps is the required half of ui.Deps with inert fakes; tests add the
// optional fields they exercise.
func baseDeps(st registry.State, terms map[string]termwrap.Terminal) ui.Deps {
	return ui.Deps{State: st, Terms: terms, Create: noCreate, Start: noStart}
}

func modelWithFakes(t *testing.T) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	return ui.NewModel(baseDeps(twoProjectState(), terms)), fakes
}

func press(m *ui.Model, k tea.KeyPressMsg) { m.Update(k) }

// pressAndSettle presses a key and then runs the work it scheduled, the way the
// bubbletea runtime does. Needed wherever a keypress deliberately hands a slow
// step to a tea.Cmd - stopping a held claude, deleting a worktree - because
// press alone drops the command and the test then asserts against a model that
// is only half-way through the operation (#43).
func pressAndSettle(m *ui.Model, k tea.KeyPressMsg) {
	_, cmd := m.Update(k)
	settle(m, cmd)
}

// settle runs cmd and feeds whatever it produces back into the model until
// nothing is left, unwrapping batches the way the runtime does.
func settle(m *ui.Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			settle(m, c)
		}
	default:
		_, next := m.Update(msg)
		settle(m, next)
	}
}

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

	if got := m.Selected(); got != "s2" {
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

	if got := m.Selected(); got != "s1" {
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

// Only the *selected* terminal is resized on a window change. Selected, not
// focused: #95 split the two, because a resize must reach the pane's terminal
// whether or not a modal currently owns the keyboard.; the others catch
// up when focused (issue #73). Sizing itself is PTYSize, tested under #35/#75.
func TestModel_WindowResizeReachesOnlyTheSelectedTerminal(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if fakes["s1"].Width == 0 {
		t.Error("the focused terminal was not resized")
	}
	if fakes["s2"].Width != 0 {
		t.Errorf("an unfocused terminal was resized to %d, want untouched", fakes["s2"].Width)
	}
}

func TestModel_NoSessionsRendersWithoutPanicking(t *testing.T) {
	m := ui.NewModel(ui.Deps{State: emptyState(), Terms: map[string]termwrap.Terminal{}, Create: noCreate, Start: noStart})

	press(m, special(tea.KeyEscape))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if got := m.Selected(); got != "" {
		t.Errorf("Focused() = %q on an empty registry, want \"\"", got)
	}
	if m.View().Content == "" {
		t.Error("View() is empty on an empty registry, want at least a hint")
	}
}

// Regression, issue #73: only the focused terminal tracked the window, and a
// focus change did not resize the terminal just focused, so j/k onto a
// session showed claude painted at its birth width inside a wider box - the
// #51 symptom again.
func TestModel_MovingTheCursorResizesTheNewlySelectedTerminal_issue73(t *testing.T) {
	m, fakes := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	press(m, ctrl('o'))
	press(m, key('j'))

	if f := fakes["s2"]; f.Width != 90 || f.Height != 36 {
		t.Errorf("newly focused s2 is %dx%d, want PTYSize(120,40) = 90x36", f.Width, f.Height)
	}
}
