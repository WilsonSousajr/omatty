package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Line one: rail, glyph, space, 18 columns of title, space, the age in four,
// a blank. Line two: rail, two spaces, 16 columns for the branch and the
// diffstat, a space, the six-cell lane, a blank. Both 27 cells.
func TestCard_HasTheSpecsColumns_issue176(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	status(m, "s1", watcher.TurnEnded, time.Now().Add(-4*time.Minute))

	card := m.CardOf("s1")

	if len(card) != 2 {
		t.Fatalf("a card is %d lines, want 2", len(card))
	}
	one, two := stripSGR(card[0]), stripSGR(card[1])
	if want := "▎✓ main" + strings.Repeat(" ", 14) + "   4m "; one != want {
		t.Errorf("line one = %q\nwant       %q", one, want)
	}
	if want := "▎  " + strings.Repeat(" ", 16) + " " + "     ▂" + " "; two != want {
		t.Errorf("line two = %q\nwant       %q", two, want)
	}
	for i, l := range card {
		if lipgloss.Width(l) != ui.SidebarWidth-1 {
			t.Errorf("line %d is %d cells, want %d", i, lipgloss.Width(l), ui.SidebarWidth-1)
		}
	}
}

func TestCard_TheRailIsAccentOnTheSelectedCardOnly_issue176(t *testing.T) {
	m, _ := modelWithFakes(t)
	rail := ui.Rail()
	if sel := m.CardOf("s1"); !strings.HasPrefix(sel[0], rail) || !strings.HasPrefix(sel[1], rail) {
		t.Errorf("the selected card does not open both lines with the accent rail: %q", sel)
	}
	if other := m.CardOf("s2"); strings.Contains(other[0], rail) || !strings.HasPrefix(stripSGR(other[0]), " ") {
		t.Errorf("an unselected card carries the rail: %q", other)
	}
}

func TestCard_TheTitleIsClippedToEighteenColumns_issue176(t *testing.T) {
	st := twoProjectState()
	st.Sessions[0].Title = "a-title-that-is-far-longer-than-eighteen"
	m := ui.NewModel(baseDeps(st, fakeTermsFor(st)))
	one := stripSGR(m.CardOf("s1")[0])
	if !strings.Contains(one, "a-title-that-is-fa") || strings.Contains(one, "far-l") {
		t.Errorf("line one = %q, want the title cut at 18 columns", one)
	}
}

// A project header is one line: the rail column, then the name, muted; the
// rail only on an empty project the cursor rests on (#158).
func TestCard_AProjectHeaderIsOneLine_issue176(t *testing.T) {
	m := modelWithEmptyProject(t, &recordCreate{})
	lines := frameLines(m)
	if got := stripSGR(lines[2]); !strings.HasPrefix(got, " omatty") {
		t.Errorf("the first body line = %q, want the omatty header with no rail", got)
	}
	leader(m, key(']'))
	if got := stripSGR(m.View().Content); !strings.Contains(got, "▎wstech") {
		t.Errorf("the selected empty project's header carries no rail:\n%s", got)
	}
}

// The age is on the card and in the header row both; #128 moved it off the
// one-line row for the title's sake, and the two-line card has the room.
func TestCard_TheAgeIsOnTheCardAndInTheHeader_issue176(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	status(m, "s1", watcher.TurnEnded, time.Now().Add(-4*time.Minute))

	if head := stripSGR(frameLines(m)[0]); !strings.Contains(head, "4m") {
		t.Errorf("the header row does not carry the age: %q", head)
	}
	if one := stripSGR(m.CardOf("s1")[0]); !strings.HasSuffix(one, "  4m ") {
		t.Errorf("line one = %q, want the age right-aligned before the blank column", one)
	}
}
