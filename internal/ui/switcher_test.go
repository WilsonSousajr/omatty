package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// openSwitcher opens the fuzzy switcher and types query into it.
func openSwitcher(m *ui.Model, query string) {
	press(m, ctrl('o'))
	press(m, key('/'))
	for _, c := range query {
		press(m, key(c))
	}
}

// The point of the switcher: reach a session in another project without
// walking every row in between. twoProjectState puts s3 in api-svc, two
// sessions below the cursor's start.
func TestModel_switcherJumpsAcrossProjects_issue42(t *testing.T) {
	m, _ := modelWithFakes(t)

	openSwitcher(m, "api")
	press(m, special(tea.KeyEnter))

	if got := m.Focused(); got != "s3" {
		t.Errorf("focused session = %q after jumping to api-svc, want s3", got)
	}
}

// selectSession could only ever walk downward, so a jump backwards is the case
// that proves SelectByID replaced it (#42).
func TestModel_switcherJumpsBackwards_issue42(t *testing.T) {
	m, _ := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('j'))
	press(m, ctrl('o'))
	press(m, key('j')) // now on s3, the last row
	if m.Focused() != "s3" {
		t.Fatalf("precondition: cursor is on %q, want s3", m.Focused())
	}

	openSwitcher(m, "parser")
	press(m, special(tea.KeyEnter))

	if got := m.Focused(); got != "s2" {
		t.Errorf("focused session = %q after jumping back to parser-fix, want s2", got)
	}
}

// Landing somewhere must size it, or claude paints at the wrong width there.
func TestModel_switcherSizesTheSessionItLandsOn_issue42(t *testing.T) {
	m, fakes := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	openSwitcher(m, "api")
	press(m, special(tea.KeyEnter))

	wantW, wantH := ui.PTYSize(120, 40, false)
	if f := fakes["s3"]; f.Width != wantW || f.Height != wantH {
		t.Errorf("s3 is %dx%d after jumping to it, want %dx%d", f.Width, f.Height, wantW, wantH)
	}
}

// The list must narrow as you type, and say how much of it is left.
func TestModel_switcherFiltersAsYouType_issue42(t *testing.T) {
	m, _ := modelWithFakes(t)

	openSwitcher(m, "")
	all := m.View().Content
	if !strings.Contains(all, "3 of 3") {
		t.Fatalf("an empty query does not show every session:\n%s", all)
	}

	// "par" rather than "p": the filter searches the project name as well as
	// the title, and p alone also matches "main" in api-svc.
	for _, c := range "par" {
		press(m, key(c))
	}

	got := m.View().Content
	if !strings.Contains(got, "1 of 3") {
		t.Errorf("typing par did not narrow the list to parser-fix:\n%s", got)
	}
}

// Searching the project name as well as the title is what makes the switcher
// useful across projects: you rarely remember which repo "main" was in.
func TestModel_switcherMatchesOnTheProjectName_issue42(t *testing.T) {
	m, _ := modelWithFakes(t)

	openSwitcher(m, "svc")

	if got := m.View().Content; !strings.Contains(got, "1 of 3") {
		t.Errorf("a query on the project name matched %s, want just the api-svc session:\n%s",
			"something else", got)
	}
}

// j and k are filter text here, not movement: a switcher you cannot type "jk"
// into is not a switcher (#42).
func TestModel_switcherTreatsJAndKAsFilterText_issue42(t *testing.T) {
	m, _ := modelWithFakes(t)

	openSwitcher(m, "jk")

	got := m.View().Content
	if !strings.Contains(got, "jk") {
		t.Errorf("j and k did not reach the query:\n%s", got)
	}
	if !strings.Contains(got, "0 of 3") {
		t.Errorf("the query jk matched something:\n%s", got)
	}
}

func TestModel_switcherMovesWithCtrlJAndCtrlK_issue42(t *testing.T) {
	m, _ := modelWithFakes(t)

	openSwitcher(m, "")
	press(m, ctrl('j')) // onto the second match
	press(m, special(tea.KeyEnter))

	if got := m.Focused(); got != "s2" {
		t.Errorf("focused session = %q after ctrl+j then enter, want s2", got)
	}
}

// A query matching nothing must leave the list open rather than closing on a
// choice that was never made.
func TestModel_switcherEnterOnNoMatchesKeepsTheListOpen_issue42(t *testing.T) {
	m, _ := modelWithFakes(t)
	openSwitcher(m, "zzz")

	press(m, special(tea.KeyEnter))

	if got := m.View().Content; !strings.Contains(got, "jump to session") {
		t.Errorf("the switcher closed on a query that matched nothing:\n%s", got)
	}
}

func TestModel_switcherEscLeavesTheCursorAlone_issue42(t *testing.T) {
	m, _ := modelWithFakes(t)
	before := m.Focused()

	openSwitcher(m, "api")
	press(m, special(tea.KeyEscape))

	if got := m.Focused(); got != before {
		t.Errorf("focused session = %q after esc, want the original %q", got, before)
	}
}

// With no sessions there is nothing to switch to, so the key must do nothing
// rather than open an empty box the operator has to escape.
func TestModel_switcherDoesNotOpenWithNoSessions_issue42(t *testing.T) {
	m := ui.NewModel(baseDeps(emptyState(), map[string]termwrap.Terminal{}))

	press(m, ctrl('o'))
	press(m, key('/'))

	if got := m.View().Content; strings.Contains(got, "jump to session") {
		t.Errorf("the switcher opened with no sessions to show:\n%s", got)
	}
}

// Issue #28, for the switcher.
func TestModel_ctrlCQuitsWhileTheSwitcherIsOpen_issue28(t *testing.T) {
	m, _ := modelWithFakes(t)
	openSwitcher(m, "")

	_, cmd := m.Update(ctrl('c'))

	if !isQuit(cmd) {
		t.Error("ctrl+c while the switcher is open did not quit")
	}
}

// Invariant 1: the query is typed into omatty, never into Claude.
func TestModel_switcherKeysStayOutOfThePTY_issue42(t *testing.T) {
	m, fakes := modelWithFakes(t)
	before := len(fakes["s1"].Msgs)

	openSwitcher(m, "api")

	if got := len(fakes["s1"].Msgs); got != before {
		t.Errorf("the terminal received %d messages while the switcher was open, want %d",
			got, before)
	}
}
