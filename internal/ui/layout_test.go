package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// Both boxes are content plus one border column each side; the sidebar's
// outer width is fixed and the terminal takes the rest.
func TestPaneSize_SubtractsSidebarAndBorders_issue35(t *testing.T) {
	w, h := ui.PaneSize(120, 40)

	// terminal outer = 120 - 28 = 92; content = 90. Rows: 40 - footer 1 - border 2 = 37.
	if w != 90 || h != 37 {
		t.Errorf("PaneSize(120, 40) = (%d, %d), want (90, 37)", w, h)
	}
}

func TestPaneSize_FloorsOnATinyWindow_issue35(t *testing.T) {
	w, h := ui.PaneSize(30, 5)

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
func TestModel_ResizePassesPaneSizeToTheFocusedTerminal_issue35(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	f := fakes["s1"]
	// PaneSize 90x37 minus the title row (issue #75).
	if f.Width != 90 || f.Height != 36 {
		t.Errorf("terminal resized to %dx%d, want PTYSize 90x36", f.Width, f.Height)
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

	got := m.View().Content
	if !strings.Contains(got, "»") {
		t.Errorf("no row carries the » focus marker:\n%s", got)
	}
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "»") && !strings.Contains(l, "main") {
			t.Errorf("» is on %q, want the focused session's row (s1, titled main)", l)
		}
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

// Regression, issue #75: the PTY was born and resized at PaneSize, but the
// pane spends its first row on the title and renders h-1 rows, so claude's
// bottom line was always clipped.
func TestPTYSize_IsOneRowShorterThanThePane_issue75(t *testing.T) {
	w, h := ui.PTYSize(120, 40)

	if w != 90 || h != 36 {
		t.Errorf("PTYSize(120, 40) = (%d, %d), want (90, 36): PaneSize 90x37 minus the title row", w, h)
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
