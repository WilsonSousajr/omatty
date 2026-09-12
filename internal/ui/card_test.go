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
// diffstat, a space, the six-cell lane, a blank. Line three: rail, two
// spaces, the 23-cell gate strip, a blank (#230). All 27 cells.
//
// The third line was added by M9. What this test asserted before was correct
// for M8; the card is three lines now by the decision on #230, and the height
// is still a constant so the click inverse cannot drift from the renderer.
func TestCard_HasTheSpecsColumns_issue176(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	status(m, "s1", watcher.TurnEnded, time.Now().Add(-4*time.Minute))

	card := m.CardOf("s1")

	if len(card) != ui.CardLines() {
		t.Fatalf("a card is %d lines, want %d", len(card), ui.CardLines())
	}
	one, two, three := stripSGR(card[0]), stripSGR(card[1]), stripSGR(card[2])
	if want := "▎✓ main" + strings.Repeat(" ", 14) + "   4m "; one != want {
		t.Errorf("line one = %q\nwant       %q", one, want)
	}
	if want := "▎  " + strings.Repeat(" ", 16) + " " + "     ▂" + " "; two != want {
		t.Errorf("line two = %q\nwant       %q", two, want)
	}
	// No gate has run, so the strip is blank - it must read as nothing, not
	// as a gate that passed.
	if want := "▎  " + strings.Repeat(" ", 23) + " "; three != want {
		t.Errorf("line three = %q\nwant       %q", three, want)
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
	if sel := m.CardOf("s1"); !allHavePrefix(sel, rail) {
		t.Errorf("the selected card does not open every line with the accent rail: %q", sel)
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

// allHavePrefix reports whether every line of a card opens with prefix, so the
// assertion follows cardLines rather than naming each line (#230).
func allHavePrefix(lines []string, prefix string) bool {
	for _, l := range lines {
		if !strings.HasPrefix(l, prefix) {
			return false
		}
	}
	return true
}
