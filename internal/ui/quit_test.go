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
	m := ui.NewModel(emptyState(), map[string]termwrap.Terminal{}, noCreate, noStart)

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
	m := ui.NewModel(registry.State{}, map[string]termwrap.Terminal{}, noCreate, noStart)

	got := m.View().Content

	if !strings.Contains(got, "omatty add") {
		t.Errorf("empty-state hint does not mention `omatty add`:\n%s", got)
	}
}

// A project with no sessions is a different empty state: creating one will
// work, so the hint should say how.
func TestModel_projectWithNoSessionsPointsAtTheNewSessionKey_issue28(t *testing.T) {
	st := registry.State{Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}}}
	m := ui.NewModel(st, map[string]termwrap.Terminal{}, noCreate, noStart)

	got := m.View().Content

	if !strings.Contains(got, ui.Leader+" n") {
		t.Errorf("hint does not mention the new-session key:\n%s", got)
	}
}

// The quit keys must be discoverable; an operator who cannot find the exit
// has to kill the process.
func TestModel_ViewShowsHowToQuit_issue28(t *testing.T) {
	m := ui.NewModel(emptyState(), map[string]termwrap.Terminal{}, noCreate, noStart)

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

// Regression, issue #34: the terminal width reserved 64 columns for a sidebar
// rendered above it and a diff pane that does not exist, so claude was told it
// had width-64 while the rest of the screen sat blank.
func TestModel_terminalGetsTheFullWidthWhenNothingSitsBesideIt_issue34(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if got := fakes["s1"].Width; got != 100 {
		t.Errorf("terminal width = %d, want the full 100; %d columns are reserved for "+
			"panes that are not rendered", got, 100-got)
	}
	if got := fakes["s1"].Height; got >= 30 {
		t.Errorf("terminal height = %d, want less than 30 (sidebar and footer take rows)", got)
	}
}

// The sidebar and footer are rendered above and below, so they cost rows.
func TestModel_terminalHeightLeavesRoomForSidebarAndFooter_issue34(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// 4 sidebar rows (2 projects + 3 sessions is 5 lines) + 1 footer.
	if got := fakes["s1"].Height; got < 10 || got > 28 {
		t.Errorf("terminal height = %d, want a sensible remainder of 30", got)
	}
}
