package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// Regression, issue #28: with no session focused every key routes to
// command(), which had no ctrl+c case, so the program had no reachable exit.
// Bubble Tea v2 does not quit on ctrl+c by itself.
func TestModel_ctrlCQuitsWhenNoSessionIsFocused_issue28(t *testing.T) {
	m := ui.NewModel(ui.Deps{State: emptyState(), Terms: map[string]termwrap.Terminal{}, Create: noCreate, Start: noStart})

	_, cmd := m.Update(ctrl('c'))

	if !isQuit(cmd) {
		t.Fatal("ctrl+c did not quit with no session focused; the program is unquittable")
	}
}

// The same trap exists while a prompt is open: the terminal is deliberately
// unfocused there, so ctrl+c must still be an escape hatch.
func TestModel_ctrlCQuitsWhileAPromptIsOpen_issue28(t *testing.T) {
	m, _ := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('n'))
	if !m.Prompt().Active {
		t.Fatal("prompt did not open")
	}

	_, cmd := m.Update(ctrl('c'))

	if !isQuit(cmd) {
		t.Error("ctrl+c did not quit while a prompt was open")
	}
}

// Invariant 1 must not regress: with a session focused, ctrl+c belongs to
// Claude, which uses it to interrupt a turn. It must never quit omatty.
func TestModel_ctrlCStillReachesAFocusedSession_issue28(t *testing.T) {
	m, fakes := modelWithFakes(t)

	_, cmd := m.Update(ctrl('c'))

	if isQuit(cmd) {
		t.Error("ctrl+c quit omatty instead of reaching the focused session")
	}
	if len(fakes["s1"].Msgs) != 1 {
		t.Errorf("focused session received %d messages, want 1", len(fakes["s1"].Msgs))
	}
}

// With no projects registered, pressing n can only fail. Say so up front
// rather than after the failure.
func TestModel_emptyRegistryPointsAtOmattyAdd_issue28(t *testing.T) {
	m := ui.NewModel(ui.Deps{State: registry.State{}, Terms: map[string]termwrap.Terminal{}, Create: noCreate, Start: noStart})

	got := m.View().Content

	if !strings.Contains(got, "omatty add") {
		t.Errorf("empty-state hint does not mention `omatty add`:\n%s", got)
	}
}

// A project with no sessions is a different empty state: creating one will
// work, so the hint should say how.
func TestModel_projectWithNoSessionsPointsAtTheNewSessionKey_issue28(t *testing.T) {
	st := registry.State{Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}}}
	m := ui.NewModel(ui.Deps{State: st, Terms: map[string]termwrap.Terminal{}, Create: noCreate, Start: noStart})

	got := m.View().Content

	if !strings.Contains(got, ui.Leader+" n") {
		t.Errorf("hint does not mention the new-session key:\n%s", got)
	}
}

// The quit keys must be discoverable; an operator who cannot find the exit
// has to kill the process.
func TestModel_ViewShowsHowToQuit_issue28(t *testing.T) {
	m := ui.NewModel(ui.Deps{State: emptyState(), Terms: map[string]termwrap.Terminal{}, Create: noCreate, Start: noStart})

	if got := m.View().Content; !strings.Contains(got, "quit") {
		t.Errorf("View() never says how to quit:\n%s", got)
	}
}

// Regression, issue #30: with a session focused, ctrl+c belongs to Claude, so
// `ctrl+o q` is the only exit - which made it the state where the hint was
// most needed and least visible.
func TestModel_keyHintsStayVisibleWithASessionFocused_issue30(t *testing.T) {
	m, _ := modelWithFakes(t)

	got := m.View().Content

	if !strings.Contains(got, "session one") {
		t.Fatalf("precondition failed: no session is focused:\n%s", got)
	}
	if !strings.Contains(got, "quit") {
		t.Errorf("View() hides the exit while a session is focused:\n%s", got)
	}
	if !strings.Contains(got, ui.Leader+" q") {
		t.Errorf("View() does not name the working quit key:\n%s", got)
	}
}

// The footer is the keymap, so the other commands belong there too.
func TestModel_footerNamesTheNavigationKeys_issue30(t *testing.T) {
	m, _ := modelWithFakes(t)

	got := m.View().Content

	for _, want := range []string{ui.Leader + " j", ui.Leader + " n"} {
		if !strings.Contains(got, want) {
			t.Errorf("footer does not mention %q:\n%s", want, got)
		}
	}
}

// Regression, issue #34: the terminal width once reserved 64 columns for a
// sidebar rendered *above* it and a diff pane that did not exist. Since #35
// the sidebar really does sit beside the terminal, so the guard is restated:
// every column of the window is accounted for by something that is rendered -
// sidebar box, terminal box - and none by a pane that is not.
func TestModel_EveryColumnIsSpentOnARenderedPane_issue34(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// sidebar outer (28) + terminal content + its two border columns == window.
	if got := ui.SidebarWidth + fakes["s1"].Width + 2; got != 100 {
		t.Errorf("sidebar + terminal + borders = %d, want the full 100; %d columns are "+
			"reserved for something that is not rendered", got, 100-got)
	}
}

// The footer below, the borders around, and the title row inside cost rows.
// This asserted 27 until issue #75: the pane's first inner row is the title,
// so a 27-row PTY had its bottom line clipped on every frame. 26 was always
// the correct number.
func TestModel_terminalHeightLeavesRoomForFooterAndBorders_issue34(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// The footer row, two border rows and the title row come off: 30 - 4 = 26.
	if got := fakes["s1"].Height; got != 26 {
		t.Errorf("terminal height = %d, want 26 (30 minus footer, borders and title)", got)
	}
}
