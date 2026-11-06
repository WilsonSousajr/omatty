package app_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// Regression, issue #753: PaneSize floored the pane at 20 columns and the
// review column at 24, so below 49 columns - 73 with the column open - the
// parts summed past the window and the frame was drawn wider than the
// screen. However narrow the window, no row may be wider than it.
func TestRender_narrowWindowNeverOverflows_issue753(t *testing.T) {
	for _, w := range []int{20, 30, 40, 48, 60, 72} {
		for _, open := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dcols/column=%v", w, open), func(t *testing.T) {
				assertFrameFits(t, w, open)
			})
		}
	}
}

// assertFrameFits draws the frame at w columns, the review column open or
// closed, and fails for every row wider than the window.
func assertFrameFits(t *testing.T, w int, open bool) {
	t.Helper()
	m, _, _ := modelWithDiff(t)
	if open {
		leader(m, key('d'))
	}
	sized(m, w, 24)
	for y, l := range frameLines(m) {
		if got := lipgloss.Width(l); got > w {
			t.Errorf("row %d is %d cells in a %d-column window: %q", y, got, w, stripSGR(l))
		}
	}
	if pw, _ := app.PaneSize(w, 24, open); pw > w {
		t.Errorf("PaneSize(%d) = %d columns, wider than the window", w, pw)
	}
}

// Issue #753, tier 2: where the column does not fit beside the pane, the one
// with the keys takes the room right of the sidebar - the column drawn as if
// zoomed, or the session alone once esc hands the keys back.
func TestRender_narrowWindowShowsWhicheverHasTheKeys_issue753(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	sized(m, 60, 24)
	if !m.ReviewFocused() {
		t.Fatal("ctrl+o d did not give the column the keys")
	}

	if view := stripSGR(m.View().Content); strings.Contains(view, "session one") || !strings.Contains(view, "@@") {
		t.Errorf("with the column focused at 60 columns, want the diff alone beside the sidebar:\n%s", view)
	}
	press(m, special(tea.KeyEscape))
	if got := stripSGR(frameLines(m)[2]); !strings.Contains(got, "session one") {
		t.Errorf("with the session focused at 60 columns row 2 is %q, want the session pane", got)
	}
}

// Issue #753, tier 4: below 49 columns the sidebar gives way, the pane starts
// at the window's left edge, and the footer says how to get the sidebar back.
func TestRender_tinyWindowDropsTheSidebar_issue753(t *testing.T) {
	m, _ := modelWithFakes(t)
	sized(m, 40, 20)
	lines := frameLines(m)

	if got := stripSGR(lines[0]); strings.Contains(got, "projects") {
		t.Errorf("the header still carries the sidebar at 40 columns: %q", got)
	}
	if got := stripSGR(lines[2]); !strings.HasPrefix(got, "session one") {
		t.Errorf("row 2 = %q, want the session pane flush at the left edge", got)
	}
	if got := stripSGR(lines[len(lines)-1]); !strings.Contains(got, "widen for the sidebar") {
		t.Errorf("the footer does not say why the sidebar is gone: %q", got)
	}
	if x, _ := app.PaneOrigin(40); x != 0 {
		t.Errorf("PaneOrigin(40) x = %d, want 0 with no sidebar", x)
	}
}

// Issue #753: with the sidebar gone, a click in the left columns is on the
// pane, not on a sidebar row that is no longer drawn.
func TestMouse_tinyWindowClickIsNotOnTheSidebar_issue753(t *testing.T) {
	m, _ := modelWithFakes(t)
	sized(m, 40, 20)
	before := m.Selected()

	m.Update(tea.MouseClickMsg{X: 2, Y: 6, Button: tea.MouseLeft})

	if got := m.Selected(); got != before {
		t.Errorf("a click at x=2 in a 40-column window selected %q, want the selection kept at %q", got, before)
	}
}
