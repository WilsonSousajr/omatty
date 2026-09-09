package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// The sidebar's outer width is fixed and the terminal takes the rest. No box
// spends columns any more (#174): the sidebar's hairline is inside its 28.
func TestPaneSize_SubtractsSidebarAndBorders_issue35(t *testing.T) {
	w, h := ui.PaneSize(120, 40, false)

	// terminal = 120 - 28 = 92. Rows: 40 - header 1 - rule 1 - footer 1 = 37.
	if w != 92 || h != 37 {
		t.Errorf("PaneSize(120, 40) = (%d, %d), want (92, 37)", w, h)
	}
}

// The two border rows became the header row and the rule, so the pane keeps
// its rows; the two border columns are gone, so it gains two (#174).
func TestPaneSize_TheFrameSpendsTwoRowsAndNoColumns_issue174(t *testing.T) {
	w, h := ui.PaneSize(80, 24, false)
	if w != 80-ui.SidebarWidth || h != 24-3 {
		t.Errorf("PaneSize(80,24) = %dx%d, want %dx21", w, h, 80-ui.SidebarWidth)
	}
	if x, y := ui.PaneOrigin(); x != ui.SidebarWidth || y != 2 {
		t.Errorf("PaneOrigin() = (%d,%d), want (%d,2)", x, y, ui.SidebarWidth)
	}
}

func TestPaneSize_FloorsOnATinyWindow_issue35(t *testing.T) {
	w, h := ui.PaneSize(30, 5, false)

	if w != 20 || h != 4 {
		t.Errorf("PaneSize(30, 5) = (%d, %d), want the floors (20, 4)", w, h)
	}
}

// Regression, issue #35: the sidebar was rendered above the terminal, so a
// growing session list pushed the pane you were reading down the screen. The
// approved design has them side by side.
func TestModel_ViewPlacesSidebarBesideTheTerminal_issue35(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	lines := strings.Split(m.View().Content, "\n")
	var termLine string
	for _, l := range lines {
		if strings.Contains(l, "session one") {
			termLine = l
			break
		}
	}
	if termLine == "" {
		t.Fatalf("the focused terminal is not rendered:\n%s", m.View().Content)
	}
	if !strings.Contains(termLine, "│") || strings.Index(termLine, "session one") < ui.SidebarWidth {
		t.Errorf("terminal content is not to the right of a %d-column sidebar:\n%q",
			ui.SidebarWidth, termLine)
	}
}

// Replaces the #34 width assertion, which assumed nothing sat beside the
// terminal. Now the sidebar does, so the terminal gets PaneSize.
func TestModel_ResizePassesPaneSizeToTheSelectedTerminal_issue35(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	f := fakes["s1"]
	// PaneSize 92x37; the title is in the header row, so the PTY is the whole
	// pane (issue #75, #128, #174).
	if f.Width != 92 || f.Height != 37 {
		t.Errorf("terminal resized to %dx%d, want PTYSize 92x37", f.Width, f.Height)
	}
}

func TestModel_FooterSpansTheFullWidthOnTheLastLine_issue35(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	lines := strings.Split(strings.TrimRight(m.View().Content, "\n"), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "ctrl+o q quit") {
		t.Errorf("last line is not the footer: %q", last)
	}
	if w := lipgloss.Width(last); w > 100 {
		t.Errorf("footer is %d wide, wider than the 100-column window", w)
	}
}

func TestModel_FocusedSessionRowIsMarked_issue35(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	lines := strings.Split(stripSGR(m.View().Content), "\n")
	first := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "▎") {
			first = i
			break
		}
	}
	if first < 0 {
		t.Fatalf("no line carries the ▎ rail:\n%s", strings.Join(lines, "\n"))
	}
	// The rail runs down both lines of the selected card (#176).
	if !strings.Contains(lines[first], "main") || !strings.HasPrefix(lines[first+1], "▎") {
		t.Errorf("the rail is on %q / %q, want both lines of s1's card (titled main)", lines[first], lines[first+1])
	}
}

// No line of the frame may exceed the window, or the terminal wraps it and the
// borders tear.
func TestModel_NoLineExceedsTheWindowWidth_issue35(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	for i, l := range strings.Split(m.View().Content, "\n") {
		if w := lipgloss.Width(l); w > 100 {
			t.Errorf("line %d is %d wide: %q", i, w, l)
		}
	}
}

// Regression, issue #75: the PTY was born and resized at PaneSize while the
// pane spent its first row on the title and drew h-1 rows, so claude's
// bottom line was always clipped. The property is that the PTY is exactly
// what the pane draws; since #128 the title lives in the rule and that is
// the whole pane.
func TestPTYSize_IsOneRowShorterThanThePane_issue75(t *testing.T) {
	w, h := ui.PTYSize(120, 40, false)
	pw, ph := ui.PaneSize(120, 40, false)

	if w != pw || h != ph || h != 37 {
		t.Errorf("PTYSize(120, 40) = (%d, %d), want the pane's own (%d, %d)", w, h, pw, ph)
	}
}

// Regression, issue #74: off a tty bubbletea reports a 0x0 window, which
// clobbered the 80x24 default and floored every pane to 20x4.
func TestModel_IgnoresAZeroWindowSize_issue74(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 0, Height: 0})

	if fakes["s1"].Width != 0 {
		t.Errorf("a 0x0 window resized the terminal to %dx%d; it must be ignored", fakes["s1"].Width, fakes["s1"].Height)
	}
}

// #21: the review column takes two fifths of what the sidebar leaves, so a
// 100-column window keeps about 40 columns of claude.
func TestPaneSize_ReviewColumnTakesTwoFifthsOfTheRest_issue21(t *testing.T) {
	if got := ui.ReviewWidth(100, true); got != 28 {
		t.Errorf("ReviewWidth(100) = %d, want 28 ((100-28)*2/5)", got)
	}
	if got := ui.ReviewWidth(100, false); got != 0 {
		t.Errorf("ReviewWidth(closed) = %d, want 0", got)
	}
	w, h := ui.PaneSize(100, 30, true)
	// 100 - sidebar 28 - review 28 = 44 (#174: no border columns); rows unchanged.
	if w != 44 || h != 27 {
		t.Errorf("PaneSize(100, 30, open) = (%d, %d), want (44, 27)", w, h)
	}
	if w, _ := ui.PaneSize(160, 45, true); w != 80 {
		t.Errorf("PaneSize(160, 45, open) width = %d, want 80 (review 52)", w)
	}
}

func TestReviewWidth_FloorsOnANarrowWindow_issue21(t *testing.T) {
	if got := ui.ReviewWidth(60, true); got != 24 {
		t.Errorf("ReviewWidth(60) = %d, want the floor 24", got)
	}
}

// The title is in the rule and the row it used to take goes to the emulator.
func TestView_ThePaneTitleIsInTheTopBorderAndTheRowGoesToTheEmulator_issue128(t *testing.T) {
	m, fakes := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	lines := strings.Split(m.View().Content, "\n")
	if !strings.Contains(lines[0], "main") || !strings.Contains(lines[0], "projects") {
		t.Errorf("the first line does not carry the pane and sidebar titles: %q", lines[0])
	}
	w, h := ui.PaneSize(100, 30, false)
	if f := fakes["s1"]; f.Width != w || f.Height != h {
		t.Errorf("emulator is %dx%d, want the whole pane %dx%d", f.Width, f.Height, w, h)
	}
}
